package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func parseID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
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
