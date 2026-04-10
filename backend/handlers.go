package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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
	scanWg        sync.WaitGroup
	previewMu     sync.RWMutex
	previewStatus PreviewGenerationStatus
	scanMu        sync.RWMutex
	scanStatus    ScanStatus
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
		scanStatus:    ScanStatus{Status: "idle", Message: "No library scan has been started yet."},
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
	authRouter.Handle("/scan/status", api.authMiddleware(http.HandlerFunc(api.handleGetScanStatus))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/directory", api.authMiddleware(http.HandlerFunc(api.handleDirectoryListing))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/media-path", api.authMiddleware(http.HandlerFunc(api.handleGetMediaPath))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/media-path", api.authMiddleware(http.HandlerFunc(api.handleSetMediaPath))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/metadata", api.authMiddleware(http.HandlerFunc(api.handleGetSettingsMetadata))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/metadata", api.authMiddleware(http.HandlerFunc(api.handleUpdateSettingsMetadata))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/generation", api.authMiddleware(http.HandlerFunc(api.handleGetGenerationSettings))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/generation", api.authMiddleware(http.HandlerFunc(api.handleSetGenerationSettings))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/qbittorrent", api.authMiddleware(http.HandlerFunc(api.handleGetQBittorrentSettings))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/qbittorrent", api.authMiddleware(http.HandlerFunc(api.handleSetQBittorrentSettings))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/torrent-indexers", api.authMiddleware(http.HandlerFunc(api.handleListTorrentIndexers))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/settings/torrent-indexers", api.authMiddleware(http.HandlerFunc(api.handleCreateTorrentIndexer))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/settings/torrent-indexers/{id}", api.authMiddleware(http.HandlerFunc(api.handleDeleteTorrentIndexer))).Methods(http.MethodDelete, http.MethodOptions)
	authRouter.Handle("/admin/clear-database", api.authMiddleware(http.HandlerFunc(api.handleClearDatabase))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/torrents/search", api.authMiddleware(http.HandlerFunc(api.handleSearchTorrents))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/torrents/qbittorrent", api.authMiddleware(http.HandlerFunc(api.handleListQBittorrentTorrents))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/torrents/qbittorrent/add", api.authMiddleware(http.HandlerFunc(api.handleAddQBittorrentTorrent))).Methods(http.MethodPost, http.MethodOptions)
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
