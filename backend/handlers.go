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
	"image/color"
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
)

const sessionCookieName = "collectarr_session"

type API struct {
	store         *Store
	scanner       *Scanner
	logger        *slog.Logger
	logs          *LogBuffer
	rngMu         sync.Mutex
	rngSeed       int64
	thumbWg       sync.WaitGroup
	previewMu     sync.RWMutex
	previewStatus PreviewGenerationStatus
	sessMu        sync.RWMutex
	sessions      map[string]string
}

func NewAPI(store *Store, scanner *Scanner, logger *slog.Logger, logs *LogBuffer) *API {
	return &API{
		store:         store,
		scanner:       scanner,
		logger:        logger,
		logs:          logs,
		rngSeed:       time.Now().UnixNano(),
		previewStatus: PreviewGenerationStatus{Status: "idle", Message: "No preview generation has been started yet."},
		sessions:      map[string]string{},
	}
}

func (api *API) Router() http.Handler {
	router := mux.NewRouter()
	router.Use(corsMiddleware)

	authRouter := router.PathPrefix("/api").Subrouter()
	authRouter.Use(api.requestLoggingMiddleware)
	authRouter.HandleFunc("/auth/login", api.handleLogin).Methods(http.MethodPost, http.MethodOptions)
	authRouter.HandleFunc("/auth/logout", api.handleLogout).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/auth/me", api.authMiddleware(http.HandlerFunc(api.handleMe))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/auth/change-password", api.authMiddleware(http.HandlerFunc(api.handleChangePassword))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/logs", api.authMiddleware(http.HandlerFunc(api.handleGetLogs))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/videos", api.authMiddleware(http.HandlerFunc(api.handleListVideos))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/playlists", api.authMiddleware(http.HandlerFunc(api.handleListPlaylists))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/playlists", api.authMiddleware(http.HandlerFunc(api.handleCreatePlaylist))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/playlists/{id:[0-9]+}/cover", api.authMiddleware(http.HandlerFunc(api.handlePlaylistCover))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/playlists/{id:[0-9]+}", api.authMiddleware(http.HandlerFunc(api.handleGetPlaylist))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/playlists/{id:[0-9]+}", api.authMiddleware(http.HandlerFunc(api.handleUpdatePlaylist))).Methods(http.MethodPut, http.MethodOptions)
	authRouter.Handle("/playlists/{id:[0-9]+}", api.authMiddleware(http.HandlerFunc(api.handleDeletePlaylist))).Methods(http.MethodDelete, http.MethodOptions)
	authRouter.Handle("/playlists/{id:[0-9]+}/items", api.authMiddleware(http.HandlerFunc(api.handleAddPlaylistItems))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/playlists/{id:[0-9]+}/items", api.authMiddleware(http.HandlerFunc(api.handleReplacePlaylistItems))).Methods(http.MethodPut, http.MethodOptions)
	authRouter.Handle("/playlists/{id:[0-9]+}/items/{videoID:[0-9]+}", api.authMiddleware(http.HandlerFunc(api.handleRemovePlaylistItem))).Methods(http.MethodDelete, http.MethodOptions)
	authRouter.Handle("/videos/metadata/options", api.authMiddleware(http.HandlerFunc(api.handleVideoMetadataOptions))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/videos/metadata/bulk", api.authMiddleware(http.HandlerFunc(api.handleBulkUpdateVideoMetadata))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/videos/{id:[0-9]+}", api.authMiddleware(http.HandlerFunc(api.handleGetVideo))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/videos/{id:[0-9]+}/metadata", api.authMiddleware(http.HandlerFunc(api.handleUpdateVideoMetadata))).Methods(http.MethodPut, http.MethodOptions)
	authRouter.Handle("/videos/{id:[0-9]+}/progress", api.authMiddleware(http.HandlerFunc(api.handleGetWatchProgress))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/videos/{id:[0-9]+}/progress", api.authMiddleware(http.HandlerFunc(api.handleSaveWatchProgress))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/videos/continue-watching", api.authMiddleware(http.HandlerFunc(api.handleListContinueWatching))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/scan", api.authMiddleware(http.HandlerFunc(api.handleScan))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/directory", api.authMiddleware(http.HandlerFunc(api.handleDirectoryListing))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/media-path", api.authMiddleware(http.HandlerFunc(api.handleGetMediaPath))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/media-path", api.authMiddleware(http.HandlerFunc(api.handleSetMediaPath))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/metadata", api.authMiddleware(http.HandlerFunc(api.handleGetSettingsMetadata))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/metadata", api.authMiddleware(http.HandlerFunc(api.handleUpdateSettingsMetadata))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/generation", api.authMiddleware(http.HandlerFunc(api.handleGetGenerationSettings))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/generation", api.authMiddleware(http.HandlerFunc(api.handleSetGenerationSettings))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/torrent-indexers", api.authMiddleware(http.HandlerFunc(api.handleListTorrentIndexers))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/torrent-indexers", api.authMiddleware(http.HandlerFunc(api.handleCreateTorrentIndexer))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/torrent-indexers/{id}", api.authMiddleware(http.HandlerFunc(api.handleDeleteTorrentIndexer))).Methods(http.MethodDelete, http.MethodOptions)
	authRouter.Handle("/admin/clear-database", api.authMiddleware(http.HandlerFunc(api.handleClearDatabase))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/torrents/search", api.authMiddleware(http.HandlerFunc(api.handleSearchTorrents))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/torrents/history", api.authMiddleware(http.HandlerFunc(api.handleListTorrentDownloadHistory))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/torrents/history", api.authMiddleware(http.HandlerFunc(api.handleAddTorrentDownloadHistory))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/torrents/history/clear", api.authMiddleware(http.HandlerFunc(api.handleClearTorrentDownloadHistory))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/torrents/history/{id:[0-9]+}", api.authMiddleware(http.HandlerFunc(api.handleDeleteTorrentDownloadHistory))).Methods(http.MethodDelete, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/stream", api.authMiddleware(http.HandlerFunc(api.handleStreamVideo))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/thumbnail", api.authMiddleware(http.HandlerFunc(api.handleThumbnail))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/preview", api.authMiddleware(http.HandlerFunc(api.handlePreviewMetadata))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/preview-sprite", api.authMiddleware(http.HandlerFunc(api.handlePreviewSprite))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/video/{id:[0-9]+}/hover-preview", api.authMiddleware(http.HandlerFunc(api.handleHoverPreview))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/thumbnails/generate", api.authMiddleware(http.HandlerFunc(api.handleGenerateThumbnails))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/previews/generate", api.authMiddleware(http.HandlerFunc(api.handleGeneratePreviews))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/previews/status", api.authMiddleware(http.HandlerFunc(api.handleGetPreviewGenerationStatus))).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/settings", http.StatusTemporaryRedirect)
	}).Methods(http.MethodGet)

	spaHandler := newSPAHandler("/app/frontend/build")
	router.PathPrefix("/").Handler(spaHandler)

	return router
}

type spaHandler struct {
	staticPath string
	indexPath  string
	fs         http.FileSystem
}

func newSPAHandler(staticPath string) *spaHandler {
	return &spaHandler{
		staticPath: staticPath,
		indexPath:  filepath.Join(staticPath, "index.html"),
		fs:         http.Dir(staticPath),
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.staticPath, filepath.Clean(r.URL.Path))

	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		http.FileServer(h.fs).ServeHTTP(w, r)
		return
	}

	http.ServeFile(w, r, h.indexPath)
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

func (api *API) handleListPlaylists(w http.ResponseWriter, r *http.Request) {
	playlists, err := api.store.ListPlaylists()
	if err != nil {
		api.logger.Error("list playlists failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list playlists"})
		return
	}

	writeJSON(w, http.StatusOK, playlists)
}

func (api *API) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	var req PlaylistCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist payload"})
		return
	}

	playlist, err := api.store.CreatePlaylist(req.Name, req.Description, req.VideoIDs)
	if err != nil {
		switch {
		case strings.TrimSpace(req.Name) == "":
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "playlist name is required"})
		case api.store.IsUniquePlaylistNameError(err):
			writeJSON(w, http.StatusConflict, errorResponse{Error: "playlist name already exists"})
		case errors.Is(err, sql.ErrNoRows):
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "one or more videos were not found"})
		default:
			api.logger.Error("create playlist failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create playlist"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, playlist)
}

func (api *API) handleGetPlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist id"})
		return
	}

	playlist, err := api.store.GetPlaylistByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "playlist not found"})
			return
		}
		api.logger.Error("get playlist failed", "playlist_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load playlist"})
		return
	}

	writeJSON(w, http.StatusOK, playlist)
}

func (api *API) handlePlaylistCover(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist id"})
		return
	}

	coverPath, err := api.ensurePlaylistCover(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "playlist not found"})
			return
		}
		if errors.Is(err, errThumbnailFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "thumbnail generation requires ffmpeg"})
			return
		}
		api.logger.Error("ensure playlist cover failed", "playlist_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to generate playlist cover"})
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, coverPath)
}

func (api *API) handleUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist id"})
		return
	}

	var req PlaylistUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist payload"})
		return
	}

	playlist, err := api.store.UpdatePlaylist(id, req.Name, req.Description)
	if err != nil {
		switch {
		case strings.TrimSpace(req.Name) == "":
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "playlist name is required"})
		case api.store.IsUniquePlaylistNameError(err):
			writeJSON(w, http.StatusConflict, errorResponse{Error: "playlist name already exists"})
		case errors.Is(err, sql.ErrNoRows):
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "playlist not found"})
		default:
			api.logger.Error("update playlist failed", "playlist_id", id, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to update playlist"})
		}
		return
	}

	writeJSON(w, http.StatusOK, playlist)
}

func (api *API) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist id"})
		return
	}

	if err := api.store.DeletePlaylist(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "playlist not found"})
			return
		}
		api.logger.Error("delete playlist failed", "playlist_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to delete playlist"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (api *API) handleAddPlaylistItems(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist id"})
		return
	}

	var req PlaylistItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist items payload"})
		return
	}

	playlist, err := api.store.AddPlaylistItems(id, req.VideoIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "playlist or video not found"})
			return
		}
		api.logger.Error("add playlist items failed", "playlist_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to add playlist items"})
		return
	}

	writeJSON(w, http.StatusOK, playlist)
}

func (api *API) handleReplacePlaylistItems(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist id"})
		return
	}

	var req PlaylistItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist items payload"})
		return
	}

	playlist, err := api.store.ReplacePlaylistItems(id, req.VideoIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "playlist or video not found"})
			return
		}
		api.logger.Error("replace playlist items failed", "playlist_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to update playlist items"})
		return
	}

	writeJSON(w, http.StatusOK, playlist)
}

func (api *API) handleRemovePlaylistItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid playlist id"})
		return
	}

	videoID, err := parseID(mux.Vars(r)["videoID"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video id"})
		return
	}

	playlist, err := api.store.RemovePlaylistItem(id, videoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "playlist item not found"})
			return
		}
		api.logger.Error("remove playlist item failed", "playlist_id", id, "video_id", videoID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to remove playlist item"})
		return
	}

	writeJSON(w, http.StatusOK, playlist)
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

func (api *API) handleUpdateVideoMetadata(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video id"})
		return
	}

	var req VideoMetadataUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video metadata payload"})
		return
	}

	if err := api.store.UpdateVideoGroupMetadata(id, req.Tags, req.Actors); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "video not found"})
			return
		}
		api.logger.Error("update video group metadata failed", "video_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save video metadata"})
		return
	}

	group, err := api.store.GetVideoGroupByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "video not found"})
			return
		}
		api.logger.Error("reload video group after metadata update failed", "video_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load updated video metadata"})
		return
	}

	writeJSON(w, http.StatusOK, group)
}

func (api *API) handleVideoMetadataOptions(w http.ResponseWriter, r *http.Request) {
	options, err := api.store.ListMetadataOptions()
	if err != nil {
		api.logger.Error("list video metadata options failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load video metadata options"})
		return
	}

	writeJSON(w, http.StatusOK, options)
}

func (api *API) handleBulkUpdateVideoMetadata(w http.ResponseWriter, r *http.Request) {
	var req BulkVideoMetadataUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid bulk video metadata payload"})
		return
	}

	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "at least one video is required"})
		return
	}

	updatedGroups, err := api.store.BulkUpdateVideoGroupsMetadata(req.IDs, req.AddTags, req.RemoveTags, req.AddActors, req.RemoveActors)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "no matching videos found"})
			return
		}
		api.logger.Error("bulk update video metadata failed", "ids", req.IDs, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to update selected video metadata"})
		return
	}

	writeJSON(w, http.StatusOK, BulkVideoMetadataUpdateResponse{
		UpdatedCount:  len(updatedGroups),
		UpdatedGroups: updatedGroups,
	})
}

func (api *API) handleGetSettingsMetadata(w http.ResponseWriter, r *http.Request) {
	options, err := api.store.ListMetadataCatalog()
	if err != nil {
		api.logger.Error("get settings metadata options failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load settings metadata"})
		return
	}

	writeJSON(w, http.StatusOK, options)
}

func (api *API) handleUpdateSettingsMetadata(w http.ResponseWriter, r *http.Request) {
	var req SettingsMetadataUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid settings metadata payload"})
		return
	}

	options, err := api.store.UpdateMetadataCatalog(req.AddTags, req.RemoveTags, req.AddActors, req.RemoveActors)
	if err != nil {
		api.logger.Error("update settings metadata failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to update settings metadata"})
		return
	}

	writeJSON(w, http.StatusOK, options)
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

	api.logger.Info("scan completed", "media_path", mediaPath, "files_found", report.FilesFound, "inserted", report.Inserted, "updated", report.Updated, "skipped", report.Skipped)
	settings, err := api.store.GetGenerationSettings()
	if err != nil {
		api.logger.Error("load generation settings after scan failed", "error", err)
	} else if settings.AutoGenerateDuringScan {
		api.enqueueConfiguredPreviewAssets(settings)
	}

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
	api.logger.Warn("database cleared by request")
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

	api.logger.Info("media path updated", "path", normalizedPath)
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

	api.logger.Info("generation settings updated", "generate_thumbnails", req.GenerateThumbnails, "generate_scrubber_sprites", req.GenerateScrubberSprites, "generate_hover_previews", req.GenerateHoverPreviews, "auto_generate_during_scan", req.AutoGenerateDuringScan)
	writeJSON(w, http.StatusOK, GenerationSettingsResponse(req))
}

func (api *API) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid log limit"})
			return
		}
		if parsed > defaultLogEntryLimit {
			parsed = defaultLogEntryLimit
		}
		limit = parsed
	}

	writeJSON(w, http.StatusOK, LogsResponse{Entries: api.logs.List(limit)})
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

		api.logger.Error("ensure thumbnail failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to generate thumbnail"})
		return
	}

	if !generated {
		if _, err := os.Stat(thumbnailPath); err != nil {
			api.logger.Error("check thumbnail failed", "title", video.Title, "error", err)
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

	scrubberVideo, err := api.store.GetVideoForScrubber(video.Title)
	if err != nil {
		api.logger.Error("get video for scrubber failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to get video for scrubber"})
		return
	}

	preview, err := api.ensurePreviewSprite(r.Context(), scrubberVideo)
	if err != nil {
		if errors.Is(err, errPreviewFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "preview generation requires ffmpeg"})
			return
		}

		api.logger.Error("ensure preview sprite failed", "title", video.Title, "error", err)
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

	scrubberVideo, err := api.store.GetVideoForScrubber(video.Title)
	if err != nil {
		api.logger.Error("get video for scrubber failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to get video for scrubber"})
		return
	}

	_, err = api.ensurePreviewSprite(r.Context(), scrubberVideo)
	if err != nil {
		if errors.Is(err, errPreviewFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "preview generation requires ffmpeg"})
			return
		}

		api.logger.Error("load preview sprite failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load preview sprite"})
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, previewSpriteFilePath(video.Title))
}

func (api *API) handleHoverPreview(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	previewPath := hoverPreviewFilePath(video.Title)
	if _, err := os.Stat(previewPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "preview not found"})
			return
		}
		api.logger.Error("check hover preview failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load hover preview"})
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, previewPath)
}

func (api *API) handleGenerateThumbnails(w http.ResponseWriter, r *http.Request) {
	api.logger.Info("thumbnail generation enqueued")
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

func (api *API) handleGeneratePreviews(w http.ResponseWriter, r *http.Request) {
	var req PreviewGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if !req.GenerateThumbnails && !req.GenerateScrubberSprites && !req.GenerateHoverPreviews {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "at least one preview type must be selected"})
		return
	}

	titles, err := api.store.ListAllTitles()
	if err != nil {
		api.logger.Error("failed to list titles for preview generation", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start preview generation"})
		return
	}

	status, started := api.beginPreviewGeneration("manual", req, len(titles))
	if !started {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "preview generation is already running"})
		return
	}

	if len(titles) == 0 {
		api.finishPreviewGeneration("completed", "No videos available for preview generation.")
		writeJSON(w, http.StatusOK, api.currentPreviewGenerationStatus())
		return
	}

	api.logger.Info("preview generation enqueued", "generate_thumbnails", req.GenerateThumbnails, "generate_scrubber_sprites", req.GenerateScrubberSprites, "generate_hover_previews", req.GenerateHoverPreviews)
	api.thumbWg.Add(1)
	go func() {
		defer api.thumbWg.Done()
		api.generatePreviews(context.Background(), req, titles)
	}()

	writeJSON(w, http.StatusAccepted, status)
}

func (api *API) handleGetPreviewGenerationStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.currentPreviewGenerationStatus())
}

func (api *API) generatePreviews(parent context.Context, req PreviewGenerationRequest, titles []string) {
	api.logger.Info("starting preview generation", "thumbnails", req.GenerateThumbnails, "scrubber_sprites", req.GenerateScrubberSprites, "hover_previews", req.GenerateHoverPreviews, "titles", len(titles))

	for _, title := range titles {
		if req.GenerateThumbnails {
			api.updatePreviewGenerationStep(title, "Generating thumbnails")
			video, err := api.store.GetVideoForThumbnail(title)
			if err != nil {
				api.logger.Error("get video for thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, _, err := api.ensureThumbnail(parent, video); err != nil {
				api.logger.Error("generate thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if req.GenerateScrubberSprites {
			api.updatePreviewGenerationStep(title, "Generating scrubber sprites")
			video, err := api.store.GetVideoForScrubber(title)
			if err != nil {
				api.logger.Error("get video for scrubber failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensurePreviewSprite(parent, video); err != nil {
				api.logger.Error("generate scrubber sprite failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if req.GenerateHoverPreviews {
			api.updatePreviewGenerationStep(title, "Generating hover previews")
			video, err := api.store.GetVideoForPreview(title)
			if err != nil {
				api.logger.Error("get video for preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensureHoverPreview(parent, video); err != nil {
				api.logger.Error("generate hover preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		api.completePreviewGenerationVideo()
	}

	api.logger.Info("preview generation complete", "titles", len(titles))
	status := api.currentPreviewGenerationStatus()
	message := "Preview generation complete."
	if status.Errors > 0 {
		message = fmt.Sprintf("Preview generation complete with %d errors.", status.Errors)
	}
	api.finishPreviewGeneration("completed", message)
}

func (api *API) generateConfiguredPreviewAssets(parent context.Context) {
	settings, err := api.store.GetGenerationSettings()
	if err != nil {
		api.logger.Error("load generation settings failed", "error", err)
		return
	}
	if !settings.AutoGenerateDuringScan {
		return
	}
	if !settings.GenerateThumbnails && !settings.GenerateScrubberSprites && !settings.GenerateHoverPreviews {
		return
	}

	titles, err := api.store.ListAllTitles()
	if err != nil {
		api.logger.Error("failed to list titles for preview generation", "error", err)
		return
	}

	req := PreviewGenerationRequest{
		GenerateThumbnails:      settings.GenerateThumbnails,
		GenerateScrubberSprites: settings.GenerateScrubberSprites,
		GenerateHoverPreviews:   settings.GenerateHoverPreviews,
	}

	if _, started := api.beginPreviewGeneration("scan", req, len(titles)); !started {
		api.logger.Info("skipping configured preview generation because another job is already running")
		return
	}

	if len(titles) == 0 {
		api.finishPreviewGeneration("completed", "No videos available for preview generation.")
		return
	}

	api.logger.Info("starting configured preview asset generation", "titles", len(titles), "thumbnails", settings.GenerateThumbnails, "scrubber_sprites", settings.GenerateScrubberSprites, "hover_previews", settings.GenerateHoverPreviews)
	for _, title := range titles {
		if settings.GenerateThumbnails {
			api.updatePreviewGenerationStep(title, "Generating thumbnails")
			video, err := api.store.GetVideoForThumbnail(title)
			if err != nil {
				api.logger.Error("get video for thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, _, err := api.ensureThumbnail(parent, video); err != nil {
				api.logger.Error("generate thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if settings.GenerateScrubberSprites {
			api.updatePreviewGenerationStep(title, "Generating scrubber sprites")
			video, err := api.store.GetVideoForScrubber(title)
			if err != nil {
				api.logger.Error("get video for scrubber failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensurePreviewSprite(parent, video); err != nil {
				api.logger.Error("generate scrubber sprite failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if settings.GenerateHoverPreviews {
			api.updatePreviewGenerationStep(title, "Generating hover previews")
			video, err := api.store.GetVideoForPreview(title)
			if err != nil {
				api.logger.Error("get video for preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensureHoverPreview(parent, video); err != nil {
				api.logger.Error("generate hover preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		api.completePreviewGenerationVideo()
	}
	api.logger.Info("configured preview asset generation complete", "titles", len(titles))
	status := api.currentPreviewGenerationStatus()
	message := "Preview generation complete."
	if status.Errors > 0 {
		message = fmt.Sprintf("Preview generation complete with %d errors.", status.Errors)
	}
	api.finishPreviewGeneration("completed", message)
}

func (api *API) enqueueConfiguredPreviewAssets(settings GenerationSettingsResponse) {
	api.logger.Info("configured preview generation enqueued")
	api.thumbWg.Add(1)
	go func() {
		defer api.thumbWg.Done()
		api.generateConfiguredPreviewAssets(context.Background())
	}()
}

func (api *API) beginPreviewGeneration(source string, req PreviewGenerationRequest, totalVideos int) (PreviewGenerationStatus, bool) {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	if api.previewStatus.Running {
		return api.previewStatus, false
	}

	now := time.Now()
	api.previewStatus = PreviewGenerationStatus{
		Status:                  "running",
		Source:                  source,
		Running:                 true,
		GenerateThumbnails:      req.GenerateThumbnails,
		GenerateScrubberSprites: req.GenerateScrubberSprites,
		GenerateHoverPreviews:   req.GenerateHoverPreviews,
		TotalVideos:             totalVideos,
		ProcessedVideos:         0,
		Errors:                  0,
		Message:                 "Preview generation started in background.",
		StartedAt:               &now,
		CompletedAt:             nil,
	}

	return api.previewStatus, true
}

func (api *API) updatePreviewGenerationStep(title string, step string) {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	api.previewStatus.CurrentVideoTitle = title
	api.previewStatus.CurrentStep = step
	api.previewStatus.Message = fmt.Sprintf("%s (%d/%d)", step, api.previewStatus.ProcessedVideos+1, api.previewStatus.TotalVideos)
}

func (api *API) completePreviewGenerationVideo() {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	api.previewStatus.ProcessedVideos++
	api.previewStatus.CurrentStep = ""
	api.previewStatus.CurrentVideoID = 0
	api.previewStatus.CurrentVideoTitle = ""
	if api.previewStatus.Running {
		api.previewStatus.Message = fmt.Sprintf("Processed %d of %d videos.", api.previewStatus.ProcessedVideos, api.previewStatus.TotalVideos)
	}
}

func (api *API) incrementPreviewGenerationErrors() {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	api.previewStatus.Errors++
}

func (api *API) finishPreviewGeneration(status string, message string) {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	now := time.Now()
	api.previewStatus.Status = status
	api.previewStatus.Running = false
	api.previewStatus.CurrentStep = ""
	api.previewStatus.CurrentVideoID = 0
	api.previewStatus.CurrentVideoTitle = ""
	api.previewStatus.Message = message
	api.previewStatus.CompletedAt = &now
}

func (api *API) currentPreviewGenerationStatus() PreviewGenerationStatus {
	api.previewMu.RLock()
	defer api.previewMu.RUnlock()

	return api.previewStatus
}

func (api *API) generateAllThumbnails() {
	api.logger.Info("starting bulk thumbnail generation")

	titles, err := api.store.ListAllTitles()
	if err != nil {
		api.logger.Error("failed to list titles for thumbnails", "error", err)
		return
	}

	for i, title := range titles {
		api.logger.Info("generating thumbnail", "title", title, "progress", fmt.Sprintf("%d/%d", i+1, len(titles)))

		thumbnailPath := thumbnailFilePath(title)
		if _, err := os.Stat(thumbnailPath); err == nil {
			api.logger.Info("skipping cached thumbnail", "title", title)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			api.logger.Error("check cached thumbnail failed", "title", title, "error", err)
			continue
		}

		video, err := api.store.GetVideoForThumbnail(title)
		if err != nil {
			api.logger.Error("get video for thumbnail failed", "title", title, "error", err)
			continue
		}

		if _, _, err := api.ensureThumbnail(context.Background(), video); err != nil {
			if errors.Is(err, errThumbnailFFmpegMissing) {
				api.logger.Error("bulk thumbnail generation unavailable", "error", err)
				return
			}

			api.logger.Error("generate thumbnail in bulk failed", "title", title, "error", err)
		}
	}

	api.logger.Info("bulk thumbnail generation complete", "total", len(titles))
}

var errThumbnailFFmpegMissing = errors.New("ffmpeg not available")
var errPreviewFFmpegMissing = errors.New("ffmpeg not available for previews")

func (api *API) ensureThumbnail(parent context.Context, video Video) (string, bool, error) {
	thumbnailDir := thumbnailCacheDir()
	if err := os.MkdirAll(thumbnailDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create thumbnail directory: %w", err)
	}

	thumbnailPath := thumbnailFilePath(video.Title)
	if _, err := os.Stat(thumbnailPath); err == nil {
		api.logger.Info("thumbnail cache hit", "video_id", video.ID, "title", video.Title)
		return thumbnailPath, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("check thumbnail cache: %w", err)
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", false, errThumbnailFFmpegMissing
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

	api.logger.Info("thumbnail generated", "video_id", video.ID, "title", video.Title, "path", thumbnailPath)
	return thumbnailPath, true, nil
}

func (api *API) ensurePlaylistCover(parent context.Context, playlistID int64) (string, error) {
	playlist, err := api.store.GetPlaylistByID(playlistID)
	if err != nil {
		return "", err
	}

	coverDir := playlistCoverCacheDir()
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		return "", fmt.Errorf("create playlist cover directory: %w", err)
	}

	coverPath := playlistCoverFilePath(playlistID)
	if coverInfo, err := os.Stat(coverPath); err == nil {
		if !playlistCoverNeedsRefresh(coverInfo, playlist) {
			return coverPath, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check playlist cover cache: %w", err)
	}

	thumbPaths := make([]string, 0, 4)
	for _, item := range playlist.Items {
		if len(thumbPaths) == 4 {
			break
		}
		thumbnailVideo, err := api.store.GetVideoForThumbnail(item.Title)
		if err != nil {
			continue
		}
		thumbnailPath, _, err := api.ensureThumbnail(parent, thumbnailVideo)
		if err != nil {
			return "", err
		}
		thumbPaths = append(thumbPaths, thumbnailPath)
	}

	if err := writePlaylistCoverImage(coverPath, thumbPaths); err != nil {
		return "", err
	}

	return coverPath, nil
}

func playlistCoverNeedsRefresh(coverInfo os.FileInfo, playlist Playlist) bool {
	if playlist.DateUpdated != nil && coverInfo.ModTime().Before(playlist.DateUpdated.UTC()) {
		return true
	}

	for i, item := range playlist.Items {
		if i == 4 {
			break
		}
		thumbnailPath := thumbnailFilePath(item.Title)
		thumbInfo, err := os.Stat(thumbnailPath)
		if err != nil {
			return true
		}
		if coverInfo.ModTime().Before(thumbInfo.ModTime()) {
			return true
		}
	}

	return false
}

func writePlaylistCoverImage(outputPath string, thumbPaths []string) error {
	const coverWidth = 640
	const coverHeight = 360
	const columns = 2
	const rows = 2
	cellWidth := coverWidth / columns
	cellHeight := coverHeight / rows

	canvas := image.NewRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	background := image.NewUniform(color.Gray{Y: 22})
	draw.Draw(canvas, canvas.Bounds(), background, image.Point{}, draw.Src)

	blank := image.NewUniform(color.Gray{Y: 36})
	for idx := 0; idx < columns*rows; idx++ {
		x := (idx % columns) * cellWidth
		y := (idx / columns) * cellHeight
		cellRect := image.Rect(x, y, x+cellWidth, y+cellHeight)
		draw.Draw(canvas, cellRect, blank, image.Point{}, draw.Src)
	}

	for idx, thumbPath := range thumbPaths {
		if idx == columns*rows {
			break
		}
		file, err := os.Open(thumbPath)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(file)
		_ = file.Close()
		if err != nil {
			continue
		}
		x := (idx % columns) * cellWidth
		y := (idx / columns) * cellHeight
		cellRect := image.Rect(x, y, x+cellWidth, y+cellHeight)
		draw.Draw(canvas, cellRect, resizeAndCropToRect(img, cellWidth, cellHeight), image.Point{}, draw.Src)
	}

	tempPath := outputPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create playlist cover file: %w", err)
	}
	if err := jpeg.Encode(file, canvas, &jpeg.Options{Quality: 85}); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("encode playlist cover: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close playlist cover file: %w", err)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace playlist cover file: %w", err)
	}

	return nil
}

func resizeAndCropToRect(src image.Image, width, height int) image.Image {
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth == 0 || srcHeight == 0 || width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	srcAspect := float64(srcWidth) / float64(srcHeight)
	dstAspect := float64(width) / float64(height)

	var cropWidth, cropHeight int
	var offsetX, offsetY int
	if srcAspect > dstAspect {
		cropHeight = srcHeight
		cropWidth = int(float64(cropHeight) * dstAspect)
		offsetX = (srcWidth - cropWidth) / 2
	} else {
		cropWidth = srcWidth
		cropHeight = int(float64(cropWidth) / dstAspect)
		offsetY = (srcHeight - cropHeight) / 2
	}
	if cropWidth <= 0 {
		cropWidth = srcWidth
	}
	if cropHeight <= 0 {
		cropHeight = srcHeight
	}

	for y := 0; y < height; y++ {
		srcY := offsetY + (y*cropHeight)/height
		for x := 0; x < width; x++ {
			srcX := offsetX + (x*cropWidth)/width
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	return dst
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

	spritePath := previewSpriteFilePath(video.Title)
	metadataPath := previewMetadataFilePath(video.Title)
	sourceInfo, err := os.Stat(video.Path)
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("stat source video: %w", err)
	}

	if spriteInfo, err := os.Stat(spritePath); err == nil {
		if metadata, err := readPreviewMetadata(metadataPath); err == nil && spriteInfo.ModTime().After(sourceInfo.ModTime()) {
			metadata.SpriteURL = fmt.Sprintf("/api/video/%d/preview-sprite", video.ID)
			api.logger.Info("preview sprite cache hit", "video_id", video.ID, "title", video.Title)
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

	api.logger.Info("preview sprite generated", "video_id", video.ID, "title", video.Title, "path", spritePath)
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

	previewPath := hoverPreviewFilePath(video.Title)
	sourceInfo, err := os.Stat(video.Path)
	if err != nil {
		return "", fmt.Errorf("stat source video: %w", err)
	}
	if previewInfo, err := os.Stat(previewPath); err == nil && previewInfo.ModTime().After(sourceInfo.ModTime()) {
		api.logger.Info("hover preview cache hit", "video_id", video.ID, "title", video.Title)
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

	api.logger.Info("hover preview generated", "video_id", video.ID, "title", video.Title, "path", previewPath)
	return previewPath, nil
}

func thumbnailCacheDir() string {
	return "/data/previews/thumbnails"
}

func thumbnailFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(thumbnailCacheDir(), fmt.Sprintf("%s.jpg", safeTitle))
}

func previewCacheDir() string {
	return "/data/previews/scrubber-sprites"
}

func previewSpriteFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(previewCacheDir(), fmt.Sprintf("%s.jpg", safeTitle))
}

func previewMetadataFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(previewCacheDir(), fmt.Sprintf("%s.json", safeTitle))
}

func hoverPreviewCacheDir() string {
	return "/data/previews/hover-previews"
}

func hoverPreviewFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(hoverPreviewCacheDir(), fmt.Sprintf("%s.mp4", safeTitle))
}

func playlistCoverCacheDir() string {
	return "/data/previews/playlist-covers"
}

func playlistCoverFilePath(playlistID int64) string {
	return filepath.Join(playlistCoverCacheDir(), fmt.Sprintf("%d.jpg", playlistID))
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
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
			api.logger.Warn("authentication required", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}

		api.sessMu.RLock()
		username, ok := api.sessions[cookie.Value]
		api.sessMu.RUnlock()
		if !ok {
			api.logger.Warn("invalid session", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
			http.SetCookie(w, api.expiredSessionCookie())
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}

		user, err := api.store.GetUserByUsername(username)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				api.logger.Warn("session user missing", "username", username, "path", r.URL.Path)
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

func (api *API) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/api/logs" {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		api.logger.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"bytes", recorder.bytes,
			"duration_ms", time.Since(started).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	written, err := r.ResponseWriter.Write(data)
	r.bytes += written
	return written, err
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
