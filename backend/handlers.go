package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "collectarr_session"

type API struct {
	store    *Store
	scanner  *Scanner
	logger   *slog.Logger
	rngMu    sync.Mutex
	rngSeed  int64
	thumbWg  sync.WaitGroup
	sessMu   sync.RWMutex
	sessions map[string]string
}

func NewAPI(store *Store, scanner *Scanner, logger *slog.Logger) *API {
	return &API{
		store:    store,
		scanner:  scanner,
		logger:   logger,
		rngSeed:  time.Now().UnixNano(),
		sessions: map[string]string{},
	}
}

func (api *API) Router() http.Handler {
	router := mux.NewRouter()
	router.Use(corsMiddleware)

	authRouter := router.PathPrefix("/api").Subrouter()
	authRouter.HandleFunc("/auth/login", api.handleLogin).Methods(http.MethodPost, http.MethodOptions)
	authRouter.HandleFunc("/auth/logout", api.handleLogout).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/auth/me", api.authMiddleware(http.HandlerFunc(api.handleMe))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/auth/change-password", api.authMiddleware(http.HandlerFunc(api.handleChangePassword))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/videos", api.authMiddleware(http.HandlerFunc(api.handleListVideos))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/videos/{id:[0-9]+}", api.authMiddleware(http.HandlerFunc(api.handleGetVideo))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/scan", api.authMiddleware(http.HandlerFunc(api.handleScan))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/directory", api.authMiddleware(http.HandlerFunc(api.handleDirectoryListing))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/media-path", api.authMiddleware(http.HandlerFunc(api.handleGetMediaPath))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/media-path", api.authMiddleware(http.HandlerFunc(api.handleSetMediaPath))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/generation", api.authMiddleware(http.HandlerFunc(api.handleGetGenerationSettings))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/generation", api.authMiddleware(http.HandlerFunc(api.handleSetGenerationSettings))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/admin/clear-database", api.authMiddleware(http.HandlerFunc(api.handleClearDatabase))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/stream", api.authMiddleware(http.HandlerFunc(api.handleStreamVideo))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/thumbnail", api.authMiddleware(http.HandlerFunc(api.handleThumbnail))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/preview", api.authMiddleware(http.HandlerFunc(api.handlePreviewMetadata))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/preview-sprite", api.authMiddleware(http.HandlerFunc(api.handlePreviewSprite))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/hover-preview", api.authMiddleware(http.HandlerFunc(api.handleHoverPreview))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/thumbnails/generate", api.authMiddleware(http.HandlerFunc(api.handleGenerateThumbnails))).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/settings", http.StatusTemporaryRedirect)
	}).Methods(http.MethodGet)

	// Serve frontend static files
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("/app/frontend/build")))

	return router
}

func (api *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid login payload"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "username and password are required"})
		return
	}

	user, err := api.store.GetUserByUsername(req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid credentials"})
			return
		}
		api.logger.Error("load login user failed", "username", req.Username, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "login failed"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid credentials"})
		return
	}

	sessionID, err := generateSessionID()
	if err != nil {
		api.logger.Error("generate session failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "login failed"})
		return
	}

	api.sessMu.Lock()
	api.sessions[sessionID] = user.Username
	api.sessMu.Unlock()

	http.SetCookie(w, api.newSessionCookie(sessionID))
	writeJSON(w, http.StatusOK, user)
}

func (api *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		api.sessMu.Lock()
		delete(api.sessions, cookie.Value)
		api.sessMu.Unlock()
	}

	http.SetCookie(w, api.expiredSessionCookie())
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (api *API) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (api *API) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid password payload"})
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "current and new passwords are required"})
		return
	}
	if req.CurrentPassword == req.NewPassword {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "new password must be different"})
		return
	}

	storedUser, err := api.store.GetUserByUsername(user.Username)
	if err != nil {
		api.logger.Error("load change-password user failed", "username", user.Username, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to change password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(req.CurrentPassword)); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "current password is incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		api.logger.Error("hash new password failed", "username", user.Username, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to change password"})
		return
	}

	if err := api.store.UpdatePassword(user.Username, string(newHash)); err != nil {
		api.logger.Error("update password failed", "username", user.Username, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to change password"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}

func (api *API) handleListVideos(w http.ResponseWriter, r *http.Request) {
	groups, err := api.store.ListVideoGroups()
	if err != nil {
		api.logger.Error("list video groups failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list videos"})
		return
	}

	writeJSON(w, http.StatusOK, groups)
}

func (api *API) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video id"})
		return
	}

	group, err := api.store.GetVideoGroupByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "video not found"})
			return
		}
		api.logger.Error("get video group failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load video"})
		return
	}

	writeJSON(w, http.StatusOK, group)
}

func (api *API) handleScan(w http.ResponseWriter, r *http.Request) {
	mediaPath, err := api.scanMediaPath()
	if err != nil {
		api.logger.Error("resolve scan media path failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid media path configuration"})
		return
	}

	scanner := NewScanner(mediaPath, api.store, api.logger)

	report, err := scanner.ScanLibrary(r.Context())
	if err != nil {
		api.logger.Error("scan failed", "error", err, "media_path", mediaPath)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "scan failed"})
		return
	}

	api.enqueueConfiguredPreviewAssets()

	writeJSON(w, http.StatusOK, report)
}

func (api *API) handleDirectoryListing(w http.ResponseWriter, r *http.Request) {
	api.handleDirectoryListingForRoot(w, r, api.scanner.mediaPath)
}

func (api *API) handleDirectoryListingForRoot(w http.ResponseWriter, r *http.Request, rootPath string) {
	relPath, err := normalizeRelativeMediaPath(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	dirPath, err := resolvePathUnderRoot(rootPath, relPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "directory not found"})
			return
		}
		api.logger.Error("stat directory failed", "path", dirPath, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list directory"})
		return
	}
	if !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "path must be a directory"})
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		api.logger.Error("read directory failed", "path", dirPath, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list directory"})
		return
	}

	response := make([]DirectoryEntry, 0, len(entries)+1)
	if relPath != "" {
		parentPath := filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = ""
		}
		response = append(response, DirectoryEntry{
			Name:        "..",
			Path:        filepath.ToSlash(parentPath),
			IsDirectory: true,
		})
	}

	var directories []DirectoryEntry
	for _, entry := range entries {
		entryRelPath := joinRelativePath(relPath, entry.Name())
		if entry.IsDir() {
			directories = append(directories, DirectoryEntry{
				Name:        entry.Name(),
				Path:        entryRelPath,
				IsDirectory: true,
			})
		}
	}

	sort.Slice(directories, func(i, j int) bool {
		return strings.ToLower(directories[i].Name) < strings.ToLower(directories[j].Name)
	})

	response = append(response, directories...)
	writeJSON(w, http.StatusOK, response)
}

func (api *API) handleClearDatabase(w http.ResponseWriter, r *http.Request) {
	if err := api.store.ClearDatabase(); err != nil {
		api.logger.Error("clear database failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to clear database"})
		return
	}
	writeJSON(w, http.StatusOK, ClearDatabaseResponse{Status: "database cleared"})
}

func (api *API) handleGetMediaPath(w http.ResponseWriter, r *http.Request) {
	path, err := api.store.GetMediaPath()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusOK, MediaPathResponse{Path: ""})
			return
		}
		api.logger.Error("get media path failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load media path"})
		return
	}

	normalizedPath, err := api.normalizeMediaPathSetting(path)
	if err != nil {
		api.logger.Error("normalize media path failed", "error", err, "media_path", path)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid media path configuration"})
		return
	}

	writeJSON(w, http.StatusOK, MediaPathResponse{Path: normalizedPath})
}

func (api *API) handleSetMediaPath(w http.ResponseWriter, r *http.Request) {
	var req MediaPathRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid media path payload"})
		return
	}

	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "media path is required"})
		return
	}

	normalizedPath, err := api.normalizeMediaPathSetting(req.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := api.store.SetMediaPath(normalizedPath); err != nil {
		api.logger.Error("set media path failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save media path"})
		return
	}

	writeJSON(w, http.StatusOK, MediaPathResponse{Path: normalizedPath})
}

func (api *API) handleGetGenerationSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := api.store.GetGenerationSettings()
	if err != nil {
		api.logger.Error("get generation settings failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load generation settings"})
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

func (api *API) handleSetGenerationSettings(w http.ResponseWriter, r *http.Request) {
	var req GenerationSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid generation settings payload"})
		return
	}

	if err := api.store.SetGenerationSettings(req); err != nil {
		api.logger.Error("set generation settings failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save generation settings"})
		return
	}

	writeJSON(w, http.StatusOK, GenerationSettingsResponse(req))
}

func (api *API) scanMediaPath() (string, error) {
	storedPath, err := api.store.GetMediaPath()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return api.scanner.mediaPath, nil
		}
		return "", err
	}
	if strings.TrimSpace(storedPath) == "" {
		return api.scanner.mediaPath, nil
	}

	normalizedPath, err := api.normalizeMediaPathSetting(storedPath)
	if err != nil {
		return "", err
	}

	return api.resolveMediaPath(normalizedPath)
}

func (api *API) normalizeMediaPathSetting(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	if strings.HasPrefix(raw, "/") {
		mediaRoot := filepath.Clean(api.scanner.mediaPath)
		candidate := filepath.Clean(raw)
		relPath, err := filepath.Rel(mediaRoot, candidate)
		if err != nil {
			return "", fmt.Errorf("normalize media path: %w", err)
		}
		if relPath == "." {
			return "", nil
		}
		if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			return "", errors.New("media path must be inside configured media root")
		}
		return normalizeRelativeMediaPath(filepath.ToSlash(relPath))
	}

	return normalizeRelativeMediaPath(raw)
}

func (api *API) handleStreamVideo(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	file, err := os.Open(video.Path)
	if err != nil {
		api.logger.Error("open video failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "video file not found"})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		api.logger.Error("stat video failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to stream video"})
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(video.Filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, video.Filename, info.ModTime(), file)
}

func (api *API) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	thumbnailPath, generated, err := api.ensureThumbnail(r.Context(), video)
	if err != nil {
		if errors.Is(err, errThumbnailFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "thumbnail generation requires ffmpeg"})
			return
		}

		api.logger.Error("ensure thumbnail failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to generate thumbnail"})
		return
	}

	if !generated {
		if _, err := os.Stat(thumbnailPath); err != nil {
			api.logger.Error("check thumbnail failed", "id", video.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load thumbnail"})
			return
		}
	}

	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, thumbnailPath)
}

func (api *API) handlePreviewMetadata(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	preview, err := api.ensurePreviewSprite(r.Context(), video)
	if err != nil {
		if errors.Is(err, errPreviewFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "preview generation requires ffmpeg"})
			return
		}

		api.logger.Error("ensure preview sprite failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to generate preview sprite"})
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

func (api *API) handlePreviewSprite(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	_, err = api.ensurePreviewSprite(r.Context(), video)
	if err != nil {
		if errors.Is(err, errPreviewFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "preview generation requires ffmpeg"})
			return
		}

		api.logger.Error("load preview sprite failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load preview sprite"})
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, previewSpriteFilePath(video.ID))
}

func (api *API) handleHoverPreview(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	previewPath, err := api.ensureHoverPreview(r.Context(), video)
	if err != nil {
		if errors.Is(err, errPreviewFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "hover preview generation requires ffmpeg"})
			return
		}

		api.logger.Error("load hover preview failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load hover preview"})
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, previewPath)
}

func (api *API) handleGenerateThumbnails(w http.ResponseWriter, r *http.Request) {
	api.thumbWg.Add(1)
	go func() {
		defer api.thumbWg.Done()
		api.generateAllThumbnails()
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Thumbnail generation started in background",
	})
}

func (api *API) generateConfiguredPreviewAssets(parent context.Context) {
	settings, err := api.store.GetGenerationSettings()
	if err != nil {
		api.logger.Error("load generation settings failed", "error", err)
		return
	}
	if !settings.GenerateScrubberSprites && !settings.GenerateHoverPreviews {
		return
	}

	videos, err := api.store.ListVideos()
	if err != nil {
		api.logger.Error("failed to list videos for preview generation", "error", err)
		return
	}

	api.logger.Info("starting configured preview asset generation", "videos", len(videos), "scrubber_sprites", settings.GenerateScrubberSprites, "hover_previews", settings.GenerateHoverPreviews)
	for _, video := range videos {
		if settings.GenerateScrubberSprites {
			if _, err := api.ensurePreviewSprite(parent, video); err != nil && !errors.Is(err, errPreviewFFmpegMissing) {
				api.logger.Error("generate scrubber sprite failed", "video_id", video.ID, "title", video.Title, "error", err)
			}
		}
		if settings.GenerateHoverPreviews {
			if _, err := api.ensureHoverPreview(parent, video); err != nil && !errors.Is(err, errPreviewFFmpegMissing) {
				api.logger.Error("generate hover preview failed", "video_id", video.ID, "title", video.Title, "error", err)
			}
		}
	}
	api.logger.Info("configured preview asset generation complete", "videos", len(videos))
}

func (api *API) enqueueConfiguredPreviewAssets() {
	api.thumbWg.Add(1)
	go func() {
		defer api.thumbWg.Done()
		api.generateConfiguredPreviewAssets(context.Background())
	}()
}

func (api *API) generateAllThumbnails() {
	api.logger.Info("starting bulk thumbnail generation")

	videos, err := api.store.ListVideos()
	if err != nil {
		api.logger.Error("failed to list videos for thumbnails", "error", err)
		return
	}

	for i, video := range videos {
		api.logger.Info("generating thumbnail", "video_id", video.ID, "title", video.Title, "progress", fmt.Sprintf("%d/%d", i+1, len(videos)))

		thumbnailPath := thumbnailFilePath(video.ID)
		if _, err := os.Stat(thumbnailPath); err == nil {
			api.logger.Info("skipping cached thumbnail", "video_id", video.ID, "title", video.Title)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			api.logger.Error("check cached thumbnail failed", "video_id", video.ID, "error", err)
			continue
		}

		if _, _, err := api.ensureThumbnail(context.Background(), video); err != nil {
			if errors.Is(err, errThumbnailFFmpegMissing) {
				api.logger.Error("bulk thumbnail generation unavailable", "error", err)
				return
			}

			api.logger.Error("generate thumbnail in bulk failed", "video_id", video.ID, "title", video.Title, "error", err)
		}
	}

	api.logger.Info("bulk thumbnail generation complete", "total", len(videos))
}

var errThumbnailFFmpegMissing = errors.New("ffmpeg not available")
var errPreviewFFmpegMissing = errors.New("ffmpeg not available for previews")

func (api *API) ensureThumbnail(parent context.Context, video Video) (string, bool, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", false, errThumbnailFFmpegMissing
	}

	thumbnailDir := thumbnailCacheDir()
	if err := os.MkdirAll(thumbnailDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create thumbnail directory: %w", err)
	}

	thumbnailPath := thumbnailFilePath(video.ID)
	if _, err := os.Stat(thumbnailPath); err == nil {
		return thumbnailPath, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("check thumbnail cache: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	ssTime := "1"
	if video.Duration > 0 {
		minTime := float64(video.Duration) * 0.1
		maxTime := float64(video.Duration) * 0.9
		randomTime := minTime + api.randomFloat64()*(maxTime-minTime)
		ssTime = fmt.Sprintf("%.2f", randomTime)
	}

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-ss", ssTime,
		"-i", video.Path,
		"-frames:v", "1",
		"-vf", "scale=640:360",
		thumbnailPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("generate thumbnail: %w (output: %s)", err, string(output))
	}

	return thumbnailPath, true, nil
}

func (api *API) ensurePreviewSprite(parent context.Context, video Video) (PreviewSpriteResponse, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return PreviewSpriteResponse{}, errPreviewFFmpegMissing
	}

	previewDir := previewCacheDir()
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("create preview directory: %w", err)
	}

	spritePath := previewSpriteFilePath(video.ID)
	metadataPath := previewMetadataFilePath(video.ID)
	sourceInfo, err := os.Stat(video.Path)
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("stat source video: %w", err)
	}

	if spriteInfo, err := os.Stat(spritePath); err == nil {
		if metadata, err := readPreviewMetadata(metadataPath); err == nil && spriteInfo.ModTime().After(sourceInfo.ModTime()) {
			metadata.SpriteURL = fmt.Sprintf("/api/video/%d/preview-sprite", video.ID)
			return metadata, nil
		}
	}

	const frameWidth = 120
	const frameHeight = 68
	const columns = 8
	const sampleCount = 40
	rows := int(math.Ceil(float64(sampleCount) / float64(columns)))
	timestamps := api.samplePreviewTimestamps(video.Duration, sampleCount)

	tempDir, err := os.MkdirTemp(previewDir, fmt.Sprintf("video-%d-*", video.ID))
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("create preview temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	sprite := image.NewRGBA(image.Rect(0, 0, columns*frameWidth, rows*frameHeight))
	for idx, timestamp := range timestamps {
		framePath := filepath.Join(tempDir, fmt.Sprintf("frame-%02d.jpg", idx))
		if err := generatePreviewFrame(parent, ffmpegPath, video.Path, framePath, timestamp, frameWidth, frameHeight); err != nil {
			return PreviewSpriteResponse{}, err
		}

		frameFile, err := os.Open(framePath)
		if err != nil {
			return PreviewSpriteResponse{}, fmt.Errorf("open preview frame: %w", err)
		}
		frameImage, err := jpeg.Decode(frameFile)
		frameFile.Close()
		if err != nil {
			return PreviewSpriteResponse{}, fmt.Errorf("decode preview frame: %w", err)
		}

		col := idx % columns
		row := idx / columns
		draw.Draw(sprite, image.Rect(col*frameWidth, row*frameHeight, (col+1)*frameWidth, (row+1)*frameHeight), frameImage, image.Point{}, draw.Src)
	}

	spriteFile, err := os.Create(spritePath)
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("create preview sprite: %w", err)
	}
	if err := jpeg.Encode(spriteFile, sprite, &jpeg.Options{Quality: 85}); err != nil {
		spriteFile.Close()
		return PreviewSpriteResponse{}, fmt.Errorf("encode preview sprite: %w", err)
	}
	if err := spriteFile.Close(); err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("close preview sprite: %w", err)
	}

	metadata := PreviewSpriteResponse{
		SpriteURL:   fmt.Sprintf("/api/video/%d/preview-sprite", video.ID),
		FrameWidth:  frameWidth,
		FrameHeight: frameHeight,
		Columns:     columns,
		Rows:        rows,
		Timestamps:  timestamps,
		Duration:    video.Duration,
		SampleCount: sampleCount,
	}

	if err := writePreviewMetadata(metadataPath, metadata); err != nil {
		return PreviewSpriteResponse{}, err
	}

	return metadata, nil
}

func (api *API) ensureHoverPreview(parent context.Context, video Video) (string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", errPreviewFFmpegMissing
	}

	previewDir := hoverPreviewCacheDir()
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		return "", fmt.Errorf("create hover preview directory: %w", err)
	}

	previewPath := hoverPreviewFilePath(video.ID)
	sourceInfo, err := os.Stat(video.Path)
	if err != nil {
		return "", fmt.Errorf("stat source video: %w", err)
	}
	if previewInfo, err := os.Stat(previewPath); err == nil && previewInfo.ModTime().After(sourceInfo.ModTime()) {
		return previewPath, nil
	}

	segmentCount := 12
	const segmentDuration = 1.5
	if video.Duration > 0 {
		segmentCount = max(1, min(segmentCount, int(math.Ceil(float64(video.Duration)/segmentDuration))))
	}
	timestamps := sampleHoverPreviewStartTimes(video.Duration, segmentCount, segmentDuration)
	tempDir, err := os.MkdirTemp(previewDir, fmt.Sprintf("hover-%d-*", video.ID))
	if err != nil {
		return "", fmt.Errorf("create hover preview temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	segmentPaths := make([]string, 0, len(timestamps))
	for idx, timestamp := range timestamps {
		segmentPath := filepath.Join(tempDir, fmt.Sprintf("segment-%02d.mp4", idx))
		if err := generateHoverPreviewSegment(parent, ffmpegPath, video.Path, segmentPath, timestamp, segmentDuration); err != nil {
			return "", err
		}
		segmentPaths = append(segmentPaths, segmentPath)
	}

	concatFile := filepath.Join(tempDir, "segments.txt")
	var builder strings.Builder
	for _, segmentPath := range segmentPaths {
		builder.WriteString("file '")
		builder.WriteString(strings.ReplaceAll(segmentPath, "'", "'\\''"))
		builder.WriteString("'\n")
	}
	if err := os.WriteFile(concatFile, []byte(builder.String()), 0o644); err != nil {
		return "", fmt.Errorf("write hover preview concat file: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		previewPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("generate hover preview: %w (output: %s)", err, string(output))
	}

	return previewPath, nil
}

func thumbnailCacheDir() string {
	return filepath.Join(os.TempDir(), "collectarr-thumbnails")
}

func thumbnailFilePath(videoID int64) string {
	return filepath.Join(thumbnailCacheDir(), fmt.Sprintf("%d.jpg", videoID))
}

func previewCacheDir() string {
	return filepath.Join(os.TempDir(), "collectarr-preview-sprites")
}

func previewSpriteFilePath(videoID int64) string {
	return filepath.Join(previewCacheDir(), fmt.Sprintf("%d.jpg", videoID))
}

func previewMetadataFilePath(videoID int64) string {
	return filepath.Join(previewCacheDir(), fmt.Sprintf("%d.json", videoID))
}

func hoverPreviewCacheDir() string {
	return filepath.Join(os.TempDir(), "collectarr-hover-previews")
}

func hoverPreviewFilePath(videoID int64) string {
	return filepath.Join(hoverPreviewCacheDir(), fmt.Sprintf("%d-v3.mp4", videoID))
}

func generatePreviewFrame(parent context.Context, ffmpegPath, videoPath, outputPath string, timestamp float64, width, height int) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("generate preview frame: %w (output: %s)", err, string(output))
	}

	return nil
}

func generateHoverPreviewSegment(parent context.Context, ffmpegPath, videoPath, outputPath string, timestamp, segmentDuration float64) error {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-t", fmt.Sprintf("%.2f", segmentDuration),
		"-i", videoPath,
		"-an",
		"-vf", "scale=426:240",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("generate hover preview segment: %w (output: %s)", err, string(output))
	}
	return nil
}

func (api *API) samplePreviewTimestamps(duration, count int) []float64 {
	if count <= 0 {
		return nil
	}

	timestamps := make([]float64, 0, count)
	if duration <= 0 {
		for i := 0; i < count; i++ {
			timestamps = append(timestamps, float64(i+1))
		}
		return timestamps
	}

	minTime := math.Max(1, float64(duration)*0.1)
	maxTime := math.Max(minTime, float64(duration)*0.9)
	if count == 1 {
		return []float64{(minTime + maxTime) / 2}
	}
	step := (maxTime - minTime) / float64(count-1)
	for i := 0; i < count; i++ {
		timestamps = append(timestamps, minTime+(float64(i)*step))
	}
	return timestamps
}

func sampleHoverPreviewStartTimes(duration, count int, segmentDuration float64) []float64 {
	if count <= 0 {
		return nil
	}

	if duration <= 0 {
		starts := make([]float64, 0, count)
		for i := 0; i < count; i++ {
			starts = append(starts, float64(i)*segmentDuration)
		}
		return starts
	}

	maxStart := math.Max(0, float64(duration)-segmentDuration)
	if count == 1 {
		return []float64{maxStart / 2}
	}

	starts := make([]float64, 0, count)
	step := 0.0
	if count > 1 {
		step = maxStart / float64(count-1)
	}
	for i := 0; i < count; i++ {
		starts = append(starts, float64(i)*step)
	}
	return starts
}

func readPreviewMetadata(path string) (PreviewSpriteResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("read preview metadata: %w", err)
	}

	var metadata PreviewSpriteResponse
	if err := json.Unmarshal(data, &metadata); err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("decode preview metadata: %w", err)
	}

	return metadata, nil
}

func writePreviewMetadata(path string, metadata PreviewSpriteResponse) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode preview metadata: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write preview metadata: %w", err)
	}
	return nil
}

func (api *API) videoFromRequest(r *http.Request) (Video, error) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		return Video{}, err
	}
	return api.store.GetVideoByID(id)
}

func (api *API) writeVideoError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "video not found"})
		return
	}

	var numErr *strconv.NumError
	if errors.As(err, &numErr) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video id"})
		return
	}

	api.logger.Error("video lookup failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load video"})
}

func parseID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

func normalizeRelativeMediaPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\\", "/")
	if raw == "" || raw == "." || raw == "/" {
		return "", nil
	}
	if strings.HasPrefix(raw, "/") {
		return "", errors.New("path must be relative")
	}

	cleaned := filepath.Clean(raw)
	if cleaned == "." {
		return "", nil
	}

	for _, part := range strings.Split(filepath.ToSlash(cleaned), "/") {
		if part == ".." {
			return "", errors.New("path cannot contain '..'")
		}
	}

	return cleaned, nil
}

func (api *API) resolveMediaPath(relPath string) (string, error) {
	return resolvePathUnderRoot(api.scanner.mediaPath, relPath)
}

func resolvePathUnderRoot(rootPath, relPath string) (string, error) {
	mediaRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("resolve root path: %w", err)
	}

	targetPath, err := filepath.Abs(filepath.Join(mediaRoot, relPath))
	if err != nil {
		return "", fmt.Errorf("resolve media path: %w", err)
	}

	relToRoot, err := filepath.Rel(mediaRoot, targetPath)
	if err != nil {
		return "", fmt.Errorf("check media path: %w", err)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes media root")
	}

	return targetPath, nil
}

func joinRelativePath(parts ...string) string {
	joined := filepath.Join(parts...)
	if joined == "." {
		return ""
	}
	return filepath.ToSlash(joined)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}

type contextKey string

const userContextKey contextKey = "user"

func (api *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}

		api.sessMu.RLock()
		username, ok := api.sessions[cookie.Value]
		api.sessMu.RUnlock()
		if !ok {
			http.SetCookie(w, api.expiredSessionCookie())
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}

		user, err := api.store.GetUserByUsername(username)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				api.sessMu.Lock()
				delete(api.sessions, cookie.Value)
				api.sessMu.Unlock()
				http.SetCookie(w, api.expiredSessionCookie())
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
				return
			}

			api.logger.Error("load authenticated user failed", "username", username, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to validate session"})
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, User{ID: user.ID, Username: user.Username})))
	})
}

func userFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey).(User)
	return user, ok
}

func generateSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (api *API) newSessionCookie(sessionID string) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 7,
	}
}

func (api *API) expiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}
}

func (api *API) randomFloat64() float64 {
	api.rngMu.Lock()
	defer api.rngMu.Unlock()
	api.rngSeed = (api.rngSeed*1664525 + 1013904223) & 0x7fffffffffffffff
	return float64(api.rngSeed%1_000_000) / 1_000_000
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Length, Content-Range, Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
