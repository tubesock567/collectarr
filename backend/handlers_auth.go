package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func (api *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.logger.Warn("login rejected", "reason", "invalid_payload", "remote_addr", r.RemoteAddr)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid login payload"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		api.logger.Warn("login rejected", "reason", "missing_credentials", "username", req.Username, "remote_addr", r.RemoteAddr)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "username and password are required"})
		return
	}

	user, err := api.store.GetUserByUsername(req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.logger.Warn("login failed", "username", req.Username, "reason", "unknown_user", "remote_addr", r.RemoteAddr)
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid credentials"})
			return
		}
		api.logger.Error("load login user failed", "username", req.Username, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "login failed"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		api.logger.Warn("login failed", "username", req.Username, "reason", "invalid_password", "remote_addr", r.RemoteAddr)
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
	api.logger.Info("login succeeded", "username", user.Username, "remote_addr", r.RemoteAddr)
	writeJSON(w, http.StatusOK, user)
}

func (api *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	username := ""
	if user, ok := userFromContext(r.Context()); ok {
		username = user.Username
	}

	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		api.sessMu.Lock()
		delete(api.sessions, cookie.Value)
		api.sessMu.Unlock()
	}

	http.SetCookie(w, api.expiredSessionCookie())
	api.logger.Info("logout succeeded", "username", username, "remote_addr", r.RemoteAddr)
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

	api.logger.Info("password changed", "username", user.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}
