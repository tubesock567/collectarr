package main

import "testing"

func TestBuildTorznabSearchURLOverridesQueryAndAPIKey(t *testing.T) {
	indexer := TorrentIndexer{
		TorznabURL: "https://jackett.local/api/v2.0/indexers/test/results/torznab/api?t=caps&cat=5000&apikey=old&q=old",
		APIKey:     "new-key",
	}

	result, err := buildTorznabSearchURL(indexer, "severance")
	if err != nil {
		t.Fatalf("buildTorznabSearchURL returned error: %v", err)
	}

	expected := "https://jackett.local/api/v2.0/indexers/test/results/torznab/api?apikey=new-key&cat=5000&q=severance&t=search"
	if result != expected {
		t.Fatalf("unexpected url\nwant: %s\n got: %s", expected, result)
	}
}

func TestParseTorznabResultsRejectsTorznabErrors(t *testing.T) {
	xmlBody := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <error code="100" description="Incorrect API key" />
  </channel>
</rss>`)

	_, err := parseTorznabResults(TorrentIndexer{Tracker: "BrokenTracker"}, xmlBody)
	if err == nil {
		t.Fatal("expected Torznab error")
	}
	if got := err.Error(); got != "torznab error 100: Incorrect API key" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestParseTorznabResultsPreservesExplicitZeroLeechers(t *testing.T) {
	xmlBody := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel>
    <item>
      <title>Zero Leechers</title>
      <guid>https://tracker.example/details/456</guid>
      <torznab:attr name="seeders" value="9" />
      <torznab:attr name="leechers" value="0" />
      <torznab:attr name="peers" value="14" />
    </item>
  </channel>
</rss>`)

	results, err := parseTorznabResults(TorrentIndexer{Tracker: "Tracker"}, xmlBody)
	if err != nil {
		t.Fatalf("parseTorznabResults returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Leechers != 0 {
		t.Fatalf("expected explicit zero leechers, got %d", results[0].Leechers)
	}
}

func TestFirstNonEmptyURLRejectsUnsafeSchemes(t *testing.T) {
	got := firstNonEmptyURL("javascript:alert(1)", "file:///tmp/test.torrent", "https://safe.example/file.torrent")
	if got != "https://safe.example/file.torrent" {
		t.Fatalf("unexpected safe url selection: %s", got)
	}
}

func TestParseTorznabResultsExtractsExpectedFields(t *testing.T) {
	xmlBody := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel>
    <title>SampleTracker</title>
    <item>
      <title>Sample Release</title>
      <guid>https://tracker.example/details/123</guid>
      <link>magnet:?xt=urn:btih:abc123</link>
      <size>1073741824</size>
      <comments>https://tracker.example/comments/123</comments>
      <jackettindexer>JackettTracker</jackettindexer>
      <enclosure url="https://tracker.example/download/123.torrent" length="1073741824" type="application/x-bittorrent" />
      <torznab:attr name="magneturl" value="magnet:?xt=urn:btih:abc123" />
      <torznab:attr name="seeders" value="25" />
      <torznab:attr name="peers" value="31" />
      <torznab:attr name="downloadvolumefactor" value="0" />
      <torznab:attr name="tag" value="freeleech" />
    </item>
  </channel>
</rss>`)

	results, err := parseTorznabResults(TorrentIndexer{Tracker: "FallbackTracker"}, xmlBody)
	if err != nil {
		t.Fatalf("parseTorznabResults returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Title != "Sample Release" {
		t.Fatalf("unexpected title: %s", result.Title)
	}
	if result.URL != "https://tracker.example/details/123" {
		t.Fatalf("unexpected details url: %s", result.URL)
	}
	if result.DownloadURL != "magnet:?xt=urn:btih:abc123" {
		t.Fatalf("unexpected download url: %s", result.DownloadURL)
	}
	if result.Tracker != "JackettTracker" {
		t.Fatalf("unexpected tracker: %s", result.Tracker)
	}
	if result.Size != 1073741824 {
		t.Fatalf("unexpected size: %d", result.Size)
	}
	if result.Seeders != 25 {
		t.Fatalf("unexpected seeders: %d", result.Seeders)
	}
	if result.Leechers != 6 {
		t.Fatalf("unexpected leechers: %d", result.Leechers)
	}
	if !result.Freeleech {
		t.Fatal("expected freeleech result")
	}
}
