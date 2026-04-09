package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func (api *API) handleSaveWatchProgress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video id"})
		return
	}

	var req WatchProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if req.Duration <= 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "duration must be positive"})
		return
	}

	if req.Position < 0 || req.Position > req.Duration {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "position out of range"})
		return
	}

	if err := api.store.SaveWatchProgress(videoID, req.Position, req.Duration); err != nil {
		api.logger.Error("save watch progress failed", "error", err, "video_id", videoID)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save progress"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (api *API) handleGetWatchProgress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video id"})
		return
	}

	progress, err := api.store.GetWatchProgress(videoID)
	if err != nil {
		api.logger.Error("get watch progress failed", "error", err, "video_id", videoID)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to get progress"})
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

func (api *API) handleListContinueWatching(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	items, err := api.store.ListContinueWatching(limit)
	if err != nil {
		api.logger.Error("list continue watching failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list continue watching"})
		return
	}

	writeJSON(w, http.StatusOK, items)
}
