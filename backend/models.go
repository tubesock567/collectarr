package main

import "time"

type Video struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Filename    string     `json:"filename"`
	Path        string     `json:"-"`
	Quality     string     `json:"quality,omitempty"`
	Duration    int        `json:"duration"`
	DateAdded   *time.Time `json:"date_added,omitempty"`
	DateScanned *time.Time `json:"date_scanned,omitempty"`
}

type VideoVariant struct {
	ID       int64  `json:"id"`
	Quality  string `json:"quality"`
	Filename string `json:"filename"`
}

type VideoGroup struct {
	ID          int64          `json:"id"`
	Title       string         `json:"title"`
	Duration    int            `json:"duration"`
	DateAdded   *time.Time     `json:"date_added,omitempty"`
	DateScanned *time.Time     `json:"date_scanned,omitempty"`
	Variants    []VideoVariant `json:"variants"`
}

type ScannedVideo struct {
	Title    string
	Filename string
	Path     string
	Duration int
	Quality  string
}

type ScanReport struct {
	FilesFound int `json:"files_found"`
	Inserted   int `json:"inserted"`
	Updated    int `json:"updated"`
	Skipped    int `json:"skipped"`
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type DirectoryEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDirectory bool   `json:"isDirectory"`
	Size        int64  `json:"size,omitempty"`
}

type MediaPathRequest struct {
	Path string `json:"path"`
}

type MediaPathResponse struct {
	Path string `json:"path"`
}

type ClearDatabaseResponse struct {
	Status string `json:"status"`
}

type PreviewSpriteResponse struct {
	SpriteURL   string    `json:"sprite_url"`
	FrameWidth  int       `json:"frame_width"`
	FrameHeight int       `json:"frame_height"`
	Columns     int       `json:"columns"`
	Rows        int       `json:"rows"`
	Timestamps  []float64 `json:"timestamps"`
	Duration    int       `json:"duration"`
	SampleCount int       `json:"sample_count"`
}

type errorResponse struct {
	Error string `json:"error"`
}
