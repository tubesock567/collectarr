package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"
)

const qbittorrentRequestTimeout = 20 * time.Second

var (
	errQBittorrentNotConfigured = errors.New("qBittorrent settings are incomplete")
	errQBittorrentLoginFailed   = errors.New("qBittorrent login failed")
)

type QBittorrentClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

type qbittorrentWebUITorrent struct {
	Hash          string  `json:"hash"`
	Name          string  `json:"name"`
	State         string  `json:"state"`
	Progress      float64 `json:"progress"`
	Size          int64   `json:"size"`
	TotalSize     int64   `json:"total_size"`
	Downloaded    int64   `json:"downloaded"`
	Uploaded      int64   `json:"uploaded"`
	Ratio         float64 `json:"ratio"`
	NumSeeds      int     `json:"num_seeds"`
	NumLeechers   int     `json:"num_leechs"`
	DownloadSpeed int64   `json:"dlspeed"`
	UploadSpeed   int64   `json:"upspeed"`
	ETA           int64   `json:"eta"`
	SavePath      string  `json:"save_path"`
	Category      string  `json:"category"`
	Tags          string  `json:"tags"`
	AddedOn       int64   `json:"added_on"`
	CompletionOn  int64   `json:"completion_on"`
}

func (api *API) handleGetQBittorrentSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := api.store.GetQBittorrentSettings()
	if err != nil {
		api.logger.Error("get qBittorrent settings failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load qBittorrent settings"})
		return
	}

	writeJSON(w, http.StatusOK, qbittorrentSettingsResponse(settings))
}

func (api *API) handleSetQBittorrentSettings(w http.ResponseWriter, r *http.Request) {
	var req QBittorrentSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid qBittorrent settings payload"})
		return
	}

	existing, err := api.store.GetQBittorrentSettings()
	if err != nil {
		api.logger.Error("load qBittorrent settings before update failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save qBittorrent settings"})
		return
	}

	settings, err := mergeQBittorrentSettings(req, existing)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := api.store.SetQBittorrentSettings(settings); err != nil {
		api.logger.Error("set qBittorrent settings failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save qBittorrent settings"})
		return
	}

	api.logger.Info("qBittorrent settings updated", "configured", settings.BaseURL != "")
	writeJSON(w, http.StatusOK, qbittorrentSettingsResponse(settings))
}

func (api *API) handleListQBittorrentTorrents(w http.ResponseWriter, r *http.Request) {
	client, err := api.qbittorrentClient()
	if err != nil {
		if errors.Is(err, errQBittorrentNotConfigured) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "configure qBittorrent settings first"})
			return
		}
		api.logger.Error("create qBittorrent client failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load qBittorrent settings"})
		return
	}

	torrents, err := client.ListTorrents(r.Context())
	if err != nil {
		api.logger.Error("list qBittorrent torrents failed", "error", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: qbittorrentUserMessage(err, "failed to load torrents from qBittorrent")})
		return
	}

	writeJSON(w, http.StatusOK, QBittorrentTorrentsResponse{Items: torrents})
}

func (api *API) handleAddQBittorrentTorrent(w http.ResponseWriter, r *http.Request) {
	var req QBittorrentAddTorrentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid qBittorrent torrent payload"})
		return
	}

	torrentURL, err := normalizeDownloadURL(req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	client, err := api.qbittorrentClient()
	if err != nil {
		if errors.Is(err, errQBittorrentNotConfigured) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "configure qBittorrent settings first"})
			return
		}
		api.logger.Error("create qBittorrent client failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load qBittorrent settings"})
		return
	}

	if err := client.AddTorrentURL(r.Context(), torrentURL); err != nil {
		api.logger.Error("add qBittorrent torrent failed", "error", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: qbittorrentUserMessage(err, "failed to add torrent to qBittorrent")})
		return
	}

	writeJSON(w, http.StatusAccepted, QBittorrentAddTorrentResponse{Status: "queued"})
}

func (api *API) qbittorrentClient() (*QBittorrentClient, error) {
	settings, err := api.store.GetQBittorrentSettings()
	if err != nil {
		return nil, err
	}
	settings = sanitizeQBittorrentSettings(settings)
	if settings.BaseURL == "" || settings.Username == "" || settings.Password == "" {
		return nil, errQBittorrentNotConfigured
	}
	return NewQBittorrentClient(settings)
}

func NewQBittorrentClient(settings QBittorrentSettings) (*QBittorrentClient, error) {
	settings = sanitizeQBittorrentSettings(settings)
	if settings.BaseURL == "" || settings.Username == "" || settings.Password == "" {
		return nil, errQBittorrentNotConfigured
	}

	baseURL, err := normalizeQBittorrentBaseURL(settings.BaseURL)
	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	return &QBittorrentClient{
		BaseURL:  baseURL,
		Username: settings.Username,
		Password: settings.Password,
		HTTPClient: &http.Client{
			Timeout: qbittorrentRequestTimeout,
			Jar:     jar,
		},
	}, nil
}

func (c *QBittorrentClient) ListTorrents(ctx context.Context) ([]QBittorrentTorrent, error) {
	if err := c.login(ctx); err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/torrents/info", nil)
	if err != nil {
		return nil, err
	}

	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("request qBittorrent torrents: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, readQBittorrentHTTPError("list torrents", res)
	}

	var payload []qbittorrentWebUITorrent
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode qBittorrent torrents: %w", err)
	}

	torrents := make([]QBittorrentTorrent, 0, len(payload))
	for _, item := range payload {
		torrents = append(torrents, QBittorrentTorrent{
			Hash:          item.Hash,
			Name:          item.Name,
			State:         item.State,
			Progress:      item.Progress,
			Size:          item.Size,
			TotalSize:     item.TotalSize,
			Downloaded:    item.Downloaded,
			Uploaded:      item.Uploaded,
			Ratio:         item.Ratio,
			NumSeeds:      item.NumSeeds,
			NumLeechers:   item.NumLeechers,
			DownloadSpeed: item.DownloadSpeed,
			UploadSpeed:   item.UploadSpeed,
			ETA:           item.ETA,
			SavePath:      item.SavePath,
			Category:      item.Category,
			Tags:          item.Tags,
			AddedOn:       item.AddedOn,
			CompletionOn:  item.CompletionOn,
		})
	}

	return torrents, nil
}

func (c *QBittorrentClient) AddTorrentURL(ctx context.Context, torrentURL string) error {
	if err := c.login(ctx); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("urls", torrentURL)
	req, err := c.newFormRequest(ctx, http.MethodPost, "/api/v2/torrents/add", form)
	if err != nil {
		return err
	}

	res, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("request qBittorrent add torrent: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return readQBittorrentHTTPError("add torrent", res)
	}

	return nil
}

func (c *QBittorrentClient) login(ctx context.Context) error {
	form := url.Values{}
	form.Set("username", c.Username)
	form.Set("password", c.Password)

	req, err := c.newFormRequest(ctx, http.MethodPost, "/api/v2/auth/login", form)
	if err != nil {
		return err
	}

	res, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("request qBittorrent login: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 2048))
	if err != nil {
		return fmt.Errorf("read qBittorrent login response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", errQBittorrentLoginFailed, strings.TrimSpace(string(body)))
	}

	if !strings.EqualFold(strings.TrimSpace(string(body)), "ok.") {
		return fmt.Errorf("%w: %s", errQBittorrentLoginFailed, strings.TrimSpace(string(body)))
	}

	return nil
}

func (c *QBittorrentClient) newRequest(ctx context.Context, method string, endpoint string, body io.Reader) (*http.Request, error) {
	requestURL, err := c.endpointURL(endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, err
	}
	if origin := c.originURL(); origin != "" {
		req.Header.Set("Origin", origin)
	}
	if referer := c.refererURL(); referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func (c *QBittorrentClient) newFormRequest(ctx context.Context, method string, endpoint string, form url.Values) (*http.Request, error) {
	req, err := c.newRequest(ctx, method, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func (c *QBittorrentClient) endpointURL(endpoint string) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("parse qBittorrent base URL: %w", err)
	}

	base.Path = strings.TrimRight(base.Path, "/") + endpoint
	base.RawPath = ""
	return base.String(), nil
}

func (c *QBittorrentClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *QBittorrentClient) originURL() string {
	parsed, err := url.Parse(c.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (c *QBittorrentClient) refererURL() string {
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/") {
		return base
	}
	return base + "/"
}

func normalizeQBittorrentBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("qBittorrent URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid qBittorrent URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("qBittorrent URL must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("qBittorrent URL must include host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("qBittorrent URL cannot include query parameters or fragments")
	}

	cleanPath := strings.TrimSpace(parsed.Path)
	if idx := strings.Index(strings.ToLower(cleanPath), "/api/v2"); idx >= 0 {
		cleanPath = cleanPath[:idx]
	}
	cleanPath = strings.TrimRight(cleanPath, "/")
	if cleanPath != "" {
		cleanPath = path.Clean(cleanPath)
		if cleanPath == "." || cleanPath == "/" {
			cleanPath = ""
		}
	}

	parsed.Path = cleanPath
	parsed.RawPath = ""

	return parsed.String(), nil
}

func normalizeDownloadURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("torrent URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errors.New("torrent URL is invalid")
	}
	if !isAllowedDownloadScheme(parsed.Scheme) {
		return "", errors.New("torrent URL must use http, https, or magnet")
	}
	return trimmed, nil
}

func mergeQBittorrentSettings(req QBittorrentSettingsRequest, existing QBittorrentSettings) (QBittorrentSettings, error) {
	req = QBittorrentSettingsRequest{
		BaseURL:  strings.TrimSpace(req.BaseURL),
		Username: strings.TrimSpace(req.Username),
		Password: strings.TrimSpace(req.Password),
	}
	existing = sanitizeQBittorrentSettings(existing)

	if req.BaseURL == "" && req.Username == "" && req.Password == "" {
		return QBittorrentSettings{}, nil
	}

	settings := QBittorrentSettings{
		BaseURL:  req.BaseURL,
		Username: req.Username,
		Password: req.Password,
	}
	if settings.BaseURL == "" {
		settings.BaseURL = existing.BaseURL
	}
	if settings.Username == "" {
		settings.Username = existing.Username
	}
	if settings.Password == "" {
		settings.Password = existing.Password
	}

	if settings.BaseURL == "" {
		return QBittorrentSettings{}, errors.New("qBittorrent URL is required")
	}
	if settings.Username == "" {
		return QBittorrentSettings{}, errors.New("qBittorrent username is required")
	}
	if settings.Password == "" {
		return QBittorrentSettings{}, errors.New("qBittorrent password is required")
	}

	baseURL, err := normalizeQBittorrentBaseURL(settings.BaseURL)
	if err != nil {
		return QBittorrentSettings{}, err
	}
	settings.BaseURL = baseURL

	return sanitizeQBittorrentSettings(settings), nil
}

func qbittorrentSettingsResponse(settings QBittorrentSettings) QBittorrentSettingsResponse {
	settings = sanitizeQBittorrentSettings(settings)
	return QBittorrentSettingsResponse{
		BaseURL:        settings.BaseURL,
		Username:       settings.Username,
		HasPassword:    settings.Password != "",
		MaskedPassword: maskSecret(settings.Password),
	}
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return strings.Repeat("•", len(trimmed))
	}
	return trimmed[:2] + strings.Repeat("•", len(trimmed)-4) + trimmed[len(trimmed)-2:]
}

func qbittorrentUserMessage(err error, fallback string) string {
	if errors.Is(err, errQBittorrentLoginFailed) {
		return "failed to authenticate with qBittorrent; check the saved URL and credentials"
	}
	return fallback
}

func readQBittorrentHTTPError(action string, res *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		trimmed = http.StatusText(res.StatusCode)
	}
	if res.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: %s", errQBittorrentLoginFailed, trimmed)
	}
	return fmt.Errorf("qBittorrent %s failed with status %d: %s", action, res.StatusCode, trimmed)
}
