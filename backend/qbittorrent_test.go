package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeQBittorrentBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "adds default scheme", raw: "qb.local:8080", want: "http://qb.local:8080"},
		{name: "strips api path", raw: "https://qb.local/base/api/v2/auth/login", want: "https://qb.local/base"},
		{name: "rejects query", raw: "https://qb.local:8080/?bad=1", wantErr: "cannot include query parameters or fragments"},
		{name: "rejects scheme", raw: "ftp://qb.local", wantErr: "must use http or https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeQBittorrentBaseURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeQBittorrentBaseURL returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected url\nwant: %s\n got: %s", tt.want, got)
			}
		})
	}
}

func TestMergeQBittorrentSettingsPreservesExistingPassword(t *testing.T) {
	existing := QBittorrentSettings{
		BaseURL:  "http://qb.local:8080",
		Username: "admin",
		Password: "secret123",
	}

	settings, err := mergeQBittorrentSettings(QBittorrentSettingsRequest{BaseURL: "qb.local:8080", Username: "collector"}, existing)
	if err != nil {
		t.Fatalf("mergeQBittorrentSettings returned error: %v", err)
	}
	if settings.Password != existing.Password {
		t.Fatalf("expected password to be preserved, got %q", settings.Password)
	}
	if settings.Username != "collector" {
		t.Fatalf("unexpected username: %s", settings.Username)
	}
	if settings.BaseURL != "http://qb.local:8080" {
		t.Fatalf("unexpected base url: %s", settings.BaseURL)
	}
}

func TestNormalizeDownloadURLRejectsUnsafeScheme(t *testing.T) {
	if _, err := normalizeDownloadURL("javascript:alert(1)"); err == nil {
		t.Fatal("expected invalid scheme error")
	}
}

func TestQBittorrentClientListAndAddTorrent(t *testing.T) {
	loginCalls := 0
	listCalls := 0
	addCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			loginCalls++
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse login form: %v", err)
			}
			if r.Form.Get("username") != "admin" || r.Form.Get("password") != "secret" {
				t.Fatalf("unexpected login credentials: %v", r.Form)
			}
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session", Path: "/"})
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/info":
			listCalls++
			if _, err := r.Cookie("SID"); err != nil {
				t.Fatalf("missing qBittorrent session cookie: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"hash":"abc","name":"Ubuntu ISO","state":"downloading","progress":0.5,"size":100,"total_size":200,"downloaded":100,"uploaded":10,"ratio":0.1,"num_seeds":5,"num_leechs":2,"dlspeed":1234,"upspeed":56,"eta":120}]`))
		case "/api/v2/torrents/add":
			addCalls++
			if _, err := r.Cookie("SID"); err != nil {
				t.Fatalf("missing qBittorrent session cookie: %v", err)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse add form: %v", err)
			}
			if got := r.Form.Get("urls"); got != "magnet:?xt=urn:btih:abc123" {
				t.Fatalf("unexpected torrent url: %s", got)
			}
			_, _ = w.Write([]byte("Ok."))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewQBittorrentClient(QBittorrentSettings{
		BaseURL:  server.URL,
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("NewQBittorrentClient returned error: %v", err)
	}

	torrents, err := client.ListTorrents(context.Background())
	if err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	if len(torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(torrents))
	}
	if torrents[0].DownloadSpeed != 1234 || torrents[0].NumLeechers != 2 {
		t.Fatalf("unexpected mapped torrent: %+v", torrents[0])
	}

	if err := client.AddTorrentURL(context.Background(), "magnet:?xt=urn:btih:abc123"); err != nil {
		t.Fatalf("AddTorrentURL returned error: %v", err)
	}

	if loginCalls != 2 {
		t.Fatalf("expected 2 login calls, got %d", loginCalls)
	}
	if listCalls != 1 {
		t.Fatalf("expected 1 list call, got %d", listCalls)
	}
	if addCalls != 1 {
		t.Fatalf("expected 1 add call, got %d", addCalls)
	}
}

func TestQBittorrentClientEndpointURLPreservesBasePath(t *testing.T) {
	client := &QBittorrentClient{BaseURL: "https://qb.local/base"}
	got, err := client.endpointURL("/api/v2/torrents/info")
	if err != nil {
		t.Fatalf("endpointURL returned error: %v", err)
	}
	want := "https://qb.local/base/api/v2/torrents/info"
	if got != want {
		t.Fatalf("unexpected endpoint url\nwant: %s\n got: %s", want, got)
	}
}

func TestQBittorrentClientAddTorrentUsesFormEncoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session", Path: "/"})
			_, _ = w.Write([]byte("Ok."))
			return
		}
		if r.URL.Path != "/api/v2/torrents/add" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
			t.Fatalf("unexpected content type: %s", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse request body: %v", err)
		}
		if values.Get("urls") != "https://tracker.example/file.torrent" {
			t.Fatalf("unexpected encoded urls value: %s", values.Get("urls"))
		}
		_, _ = w.Write([]byte("Ok."))
	}))
	defer server.Close()

	client, err := NewQBittorrentClient(QBittorrentSettings{BaseURL: server.URL, Username: "admin", Password: "secret"})
	if err != nil {
		t.Fatalf("NewQBittorrentClient returned error: %v", err)
	}

	if err := client.AddTorrentURL(context.Background(), "https://tracker.example/file.torrent"); err != nil {
		t.Fatalf("AddTorrentURL returned error: %v", err)
	}
}
