package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

const torrentSearchTimeout = 20 * time.Second

type torznabFeed struct {
	Channel torznabChannel `xml:"channel"`
}

type torznabChannel struct {
	Title string        `xml:"title"`
	Error *torznabError `xml:"error"`
	Items []torznabItem `xml:"item"`
}

type torznabItem struct {
	Title       string           `xml:"title"`
	Link        string           `xml:"link"`
	GUID        string           `xml:"guid"`
	Comments    string           `xml:"comments"`
	Description string           `xml:"description"`
	PubDate     string           `xml:"pubDate"`
	Indexer     string           `xml:"jackettindexer"`
	Size        int64            `xml:"size"`
	Enclosure   torznabEnclosure `xml:"enclosure"`
	Attrs       []torznabAttr    `xml:"attr"`
}

type torznabEnclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
}

type torznabAttr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type torznabError struct {
	Code        string `xml:"code,attr"`
	Description string `xml:"description,attr"`
}

func (api *API) handleListTorrentIndexers(w http.ResponseWriter, r *http.Request) {
	indexers, err := api.store.ListTorrentIndexers()
	if err != nil {
		api.logger.Error("list torrent indexers failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load torrent indexers"})
		return
	}

	writeJSON(w, http.StatusOK, torrentIndexerResponses(indexers))
}

func (api *API) handleCreateTorrentIndexer(w http.ResponseWriter, r *http.Request) {
	var req TorrentIndexerCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid torrent indexer payload"})
		return
	}

	torznabURL, tracker, err := normalizeTorznabURL(req.TorznabURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "jackett api key is required"})
		return
	}

	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = tracker
	}

	indexers, err := api.store.AddTorrentIndexer(TorrentIndexer{
		ID:         newTorrentIndexerID(),
		Label:      label,
		TorznabURL: torznabURL,
		APIKey:     apiKey,
		Tracker:    tracker,
	})
	if err != nil {
		if err.Error() == "indexer already exists" {
			writeJSON(w, http.StatusConflict, errorResponse{Error: "indexer already exists"})
			return
		}
		api.logger.Error("create torrent indexer failed", "error", err, "tracker", tracker)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save torrent indexer"})
		return
	}

	writeJSON(w, http.StatusCreated, torrentIndexerResponses(indexers))
}

func (api *API) handleDeleteTorrentIndexer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid torrent indexer id"})
		return
	}

	indexers, err := api.store.DeleteTorrentIndexer(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "torrent indexer not found"})
			return
		}
		api.logger.Error("delete torrent indexer failed", "error", err, "indexer_id", id)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to delete torrent indexer"})
		return
	}

	writeJSON(w, http.StatusOK, torrentIndexerResponses(indexers))
}

func (api *API) handleSearchTorrents(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "search query is required"})
		return
	}

	indexers, err := api.store.ListTorrentIndexers()
	if err != nil {
		api.logger.Error("load torrent indexers for search failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load torrent indexers"})
		return
	}
	if len(indexers) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "add at least one torrent indexer first"})
		return
	}

	results, warnings, err := api.searchTorrents(r.Context(), query, indexers)
	if err != nil {
		api.logger.Error("torrent search failed", "query", query, "error", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "torrent search failed"})
		return
	}

	api.logger.Info("torrent search complete", "query", query, "results", len(results), "indexers", len(indexers))
	writeJSON(w, http.StatusOK, TorrentSearchResponse{Query: query, Results: results, Warnings: warnings})
}

func (api *API) searchTorrents(parent context.Context, query string, indexers []TorrentIndexer) ([]TorrentSearchResult, []string, error) {
	ctx, cancel := context.WithTimeout(parent, torrentSearchTimeout)
	defer cancel()

	type searchResponse struct {
		results []TorrentSearchResult
		tracker string
		err     error
	}

	responses := make(chan searchResponse, len(indexers))
	var wg sync.WaitGroup
	for _, indexer := range indexers {
		indexer := indexer
		wg.Add(1)
		go func() {
			defer wg.Done()
			results, err := api.searchTorrentIndexer(ctx, indexer, query)
			responses <- searchResponse{results: results, tracker: indexer.Tracker, err: err}
		}()
	}

	wg.Wait()
	close(responses)

	allResults := make([]TorrentSearchResult, 0)
	errs := make([]error, 0)
	warnings := make([]string, 0)
	for response := range responses {
		if response.err != nil {
			errs = append(errs, response.err)
			warnings = append(warnings, fmt.Sprintf("%s failed: %v", response.tracker, response.err))
			continue
		}
		allResults = append(allResults, response.results...)
	}

	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].Seeders == allResults[j].Seeders {
			if allResults[i].Leechers == allResults[j].Leechers {
				return strings.ToLower(allResults[i].Title) < strings.ToLower(allResults[j].Title)
			}
			return allResults[i].Leechers > allResults[j].Leechers
		}
		return allResults[i].Seeders > allResults[j].Seeders
	})

	if len(allResults) == 0 && len(errs) > 0 {
		return nil, nil, errors.Join(errs...)
	}

	if len(errs) > 0 {
		api.logger.Warn("torrent search partially failed", "query", query, "errors", len(errs), "results", len(allResults))
	}

	return allResults, warnings, nil
}

func (api *API) searchTorrentIndexer(ctx context.Context, indexer TorrentIndexer, query string) ([]TorrentSearchResult, error) {
	requestURL, err := buildTorznabSearchURL(indexer, query)
	if err != nil {
		return nil, fmt.Errorf("build torznab search url for %s: %w", indexer.Tracker, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", indexer.Tracker, err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", indexer.Tracker, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("search %s returned %d: %s", indexer.Tracker, res.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", indexer.Tracker, err)
	}

	results, err := parseTorznabResults(indexer, body)
	if err != nil {
		return nil, fmt.Errorf("parse %s response: %w", indexer.Tracker, err)
	}

	return results, nil
}

func buildTorznabSearchURL(indexer TorrentIndexer, query string) (string, error) {
	parsed, err := url.Parse(indexer.TorznabURL)
	if err != nil {
		return "", err
	}
	params := parsed.Query()
	params.Set("apikey", indexer.APIKey)
	params.Set("q", query)
	params.Set("t", "search")
	parsed.RawQuery = params.Encode()
	return parsed.String(), nil
}

func parseTorznabResults(indexer TorrentIndexer, body []byte) ([]TorrentSearchResult, error) {
	var feed torznabFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	if feed.Channel.Error != nil {
		description := strings.TrimSpace(feed.Channel.Error.Description)
		if description == "" {
			description = "torznab feed returned error"
		}
		return nil, fmt.Errorf("torznab error %s: %s", strings.TrimSpace(feed.Channel.Error.Code), description)
	}

	results := make([]TorrentSearchResult, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		attrValues := make(map[string][]string, len(item.Attrs))
		for _, attr := range item.Attrs {
			name := strings.ToLower(strings.TrimSpace(attr.Name))
			if name == "" {
				continue
			}
			attrValues[name] = append(attrValues[name], strings.TrimSpace(attr.Value))
		}

		seeders := parseInt(firstAttrValue(attrValues, "seeders"))
		leechersValue, hasLeechers := firstPresentAttrValue(attrValues, "leechers")
		leechers := parseInt(leechersValue)
		if !hasLeechers {
			peers := parseInt(firstAttrValue(attrValues, "peers"))
			if peers > seeders {
				leechers = peers - seeders
			}
		}

		detailsURL := firstNonEmptyURL(
			pickURLByScheme(item.Comments, "http", "https"),
			pickNonMagnetURL(item.Description),
			pickNonMagnetURL(item.GUID),
			pickNonMagnetURL(item.Link),
		)
		downloadURL := firstNonEmptyURL(
			firstAttrValue(attrValues, "magneturl"),
			firstAttrValue(attrValues, "downloadurl"),
			item.Enclosure.URL,
			pickURLByScheme(item.GUID, "magnet", "http", "https"),
			pickURLByScheme(item.Link, "magnet", "http", "https"),
		)
		size := item.Size
		if size <= 0 {
			size = parseInt64(firstAttrValue(attrValues, "size"))
		}
		if size <= 0 {
			size = item.Enclosure.Length
		}

		tracker := firstNonEmpty(
			item.Indexer,
			firstAttrValue(attrValues, "jackettindexer"),
			firstAttrValue(attrValues, "indexer"),
			firstAttrValue(attrValues, "tracker"),
			feed.Channel.Title,
			indexer.Tracker,
		)

		var published *time.Time
		if item.PubDate != "" {
			if t, err := time.Parse(time.RFC1123, item.PubDate); err == nil {
				published = &t
			} else if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
				published = &t
			} else if t, err := time.Parse(time.RFC3339, item.PubDate); err == nil {
				published = &t
			}
		}

		results = append(results, TorrentSearchResult{
			Title:       strings.TrimSpace(item.Title),
			URL:         detailsURL,
			DownloadURL: downloadURL,
			Tracker:     tracker,
			Size:        size,
			Seeders:     seeders,
			Leechers:    leechers,
			Freeleech:   isFreeleech(attrValues),
			Published:   published,
		})
	}

	return results, nil
}

func normalizeTorznabURL(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("torznab link is required")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid torznab link: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", errors.New("torznab link must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", "", errors.New("torznab link must include host")
	}
	params := parsed.Query()
	params.Del("apikey")
	params.Del("q")
	parsed.RawQuery = params.Encode()

	tracker := strings.TrimSpace(params.Get("indexer"))
	if tracker == "" {
		tracker = strings.TrimSpace(parsed.Hostname())
	}

	return parsed.String(), tracker, nil
}

func isFreeleech(attrs map[string][]string) bool {
	for _, value := range attrs["downloadvolumefactor"] {
		factor, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil && factor == 0 {
			return true
		}
	}
	for _, value := range attrs["freeleech"] {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "yes", "1":
			return true
		}
	}
	for _, value := range attrs["tag"] {
		if strings.EqualFold(strings.TrimSpace(value), "freeleech") {
			return true
		}
	}
	return false
}

func firstAttrValue(attrs map[string][]string, keys ...string) string {
	value, _ := firstPresentAttrValue(attrs, keys...)
	return value
}

func firstPresentAttrValue(attrs map[string][]string, keys ...string) (string, bool) {
	for _, key := range keys {
		values, ok := attrs[key]
		if !ok {
			continue
		}
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			return trimmed, true
		}
		return "", true
	}
	return "", false
}

func firstNonEmptyURL(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Scheme == "" {
			continue
		}
		if !isAllowedDownloadScheme(parsed.Scheme) {
			continue
		}
		return trimmed
	}
	return ""
}

func pickURLByScheme(raw string, schemes ...string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	for _, scheme := range schemes {
		if strings.EqualFold(parsed.Scheme, scheme) {
			return trimmed
		}
	}
	return ""
}

func pickNonMagnetURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if strings.EqualFold(parsed.Scheme, "magnet") {
		return ""
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return trimmed
	}
	return ""
}

func isAllowedDownloadScheme(scheme string) bool {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http", "https", "magnet":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseInt(raw string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(raw))
	return value
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return value
}

func newTorrentIndexerID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("idx-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func torrentIndexerResponses(indexers []TorrentIndexer) []TorrentIndexerResponse {
	responses := make([]TorrentIndexerResponse, 0, len(indexers))
	for _, indexer := range indexers {
		responses = append(responses, TorrentIndexerResponse{
			ID:           indexer.ID,
			Label:        indexer.Label,
			TorznabURL:   indexer.TorznabURL,
			Tracker:      indexer.Tracker,
			MaskedAPIKey: maskAPIKey(indexer.APIKey),
		})
	}
	return responses
}

func maskAPIKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return strings.Repeat("•", len(trimmed))
	}
	return trimmed[:4] + "••••" + trimmed[len(trimmed)-4:]
}

func (api *API) handleAddTorrentDownloadHistory(w http.ResponseWriter, r *http.Request) {
	var item TorrentDownloadHistory
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	now := time.Now()
	item.DownloadedAt = &now

	if err := api.store.AddTorrentDownloadHistory(item); err != nil {
		api.logger.Error("add torrent download history failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to record download"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (api *API) handleListTorrentDownloadHistory(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	perPage := 20
	if perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 {
			perPage = pp
		}
	}

	items, totalCount, err := api.store.ListTorrentDownloadHistory(page, perPage)
	if err != nil {
		api.logger.Error("list torrent download history failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load history"})
		return
	}

	totalPages := (totalCount + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	writeJSON(w, http.StatusOK, TorrentHistoryResponse{
		Items:       items,
		TotalCount:  totalCount,
		CurrentPage: page,
		TotalPages:  totalPages,
	})
}

func (api *API) handleClearTorrentDownloadHistory(w http.ResponseWriter, r *http.Request) {
	if err := api.store.ClearTorrentDownloadHistory(); err != nil {
		api.logger.Error("clear torrent download history failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to clear history"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}
