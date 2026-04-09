package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

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

func (api *API) handleScan(w http.ResponseWriter, r *http.Request) {
	mediaPath, err := api.scanMediaPath()
	if err != nil {
		api.logger.Error("resolve scan media path failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid media path configuration"})
		return
	}

	status, started := api.beginScan(mediaPath)
	if !started {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "library scan is already running"})
		return
	}

	api.scanWg.Add(1)
	go func() {
		defer api.scanWg.Done()
		api.runScanJob(mediaPath)
	}()

	writeJSON(w, http.StatusAccepted, status)
}

func (api *API) handleGetScanStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.currentScanStatus())
}

func (api *API) runScanJob(mediaPath string) {
	api.logger.Info("scan enqueued", "media_path", mediaPath)
	scanner := NewScanner(mediaPath, api.store, api.logger)
	report, err := scanner.ScanLibraryWithProgress(context.Background(), func(progress ScanProgress) {
		api.updateScanProgress(progress)
	})
	if err != nil {
		api.logger.Error("scan failed", "error", err, "media_path", mediaPath)
		api.finishScan("failed", fmt.Sprintf("Library scan failed: %v", err), nil)
		return
	}

	api.logger.Info("scan completed", "media_path", mediaPath, "files_found", report.FilesFound, "inserted", report.Inserted, "updated", report.Updated, "skipped", report.Skipped)
	api.finishScan("completed", "Library scan completed successfully.", &report)
	settings, err := api.store.GetGenerationSettings()
	if err != nil {
		api.logger.Error("load generation settings after scan failed", "error", err)
	} else if settings.AutoGenerateDuringScan {
		api.enqueueConfiguredPreviewAssets()
	}
}

func (api *API) beginScan(mediaPath string) (ScanStatus, bool) {
	api.scanMu.Lock()
	defer api.scanMu.Unlock()

	if api.scanStatus.Running {
		return api.scanStatus, false
	}

	now := time.Now()
	api.scanStatus = ScanStatus{
		Status:    "running",
		Running:   true,
		MediaPath: mediaPath,
		Phase:     "discovering",
		Message:   "Scanning media files...",
		StartedAt: &now,
	}

	return api.scanStatus, true
}

func (api *API) updateScanProgress(progress ScanProgress) {
	api.scanMu.Lock()
	defer api.scanMu.Unlock()

	api.scanStatus.Phase = progress.Phase
	api.scanStatus.FilesFound = progress.FilesFound
	api.scanStatus.TitlesFound = progress.TitlesFound
	api.scanStatus.ProcessedFiles = progress.ProcessedFiles
	api.scanStatus.Inserted = progress.Inserted
	api.scanStatus.Updated = progress.Updated
	api.scanStatus.Skipped = progress.Skipped
	api.scanStatus.CurrentFile = progress.CurrentFile
	api.scanStatus.CurrentTitle = progress.CurrentTitle
	if progress.Message != "" {
		api.scanStatus.Message = progress.Message
	}
}

func (api *API) finishScan(status string, message string, report *ScanReport) {
	api.scanMu.Lock()
	defer api.scanMu.Unlock()

	now := time.Now()
	api.scanStatus.Status = status
	api.scanStatus.Running = false
	api.scanStatus.Message = message
	api.scanStatus.CurrentFile = ""
	api.scanStatus.CurrentTitle = ""
	api.scanStatus.CompletedAt = &now
	if report != nil {
		api.scanStatus.Phase = "completed"
		api.scanStatus.FilesFound = report.FilesFound
		api.scanStatus.ProcessedFiles = report.Inserted + report.Updated + report.Skipped
		api.scanStatus.Inserted = report.Inserted
		api.scanStatus.Updated = report.Updated
		api.scanStatus.Skipped = report.Skipped
	}
	if status == "failed" {
		api.scanStatus.Phase = "failed"
	}
}

func (api *API) currentScanStatus() ScanStatus {
	api.scanMu.RLock()
	defer api.scanMu.RUnlock()

	return api.scanStatus
}
