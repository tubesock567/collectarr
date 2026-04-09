package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

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
