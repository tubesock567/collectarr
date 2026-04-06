package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var supportedVideoExtensions = map[string]struct{}{
	".mp4":  {},
	".mkv":  {},
	".avi":  {},
	".mov":  {},
	".webm": {},
	".m4v":  {},
}

var qualityPattern = regexp.MustCompile(`(?i)[_\-\s]?(\d{3,4}p|4k)$`)

type Scanner struct {
	mediaPath string
	store     *Store
	logger    *slog.Logger
}

func NewScanner(mediaPath string, store *Store, logger *slog.Logger) *Scanner {
	return &Scanner{mediaPath: mediaPath, store: store, logger: logger}
}

func (s *Scanner) ScanLibrary(ctx context.Context) (ScanReport, error) {
	var report ScanReport
	groupedVideos := make(map[string][]ScannedVideo)

	info, err := os.Stat(s.mediaPath)
	if err != nil {
		return report, fmt.Errorf("stat media path: %w", err)
	}
	if !info.IsDir() {
		return report, fmt.Errorf("media path is not a directory: %s", s.mediaPath)
	}

	s.logger.Info("starting media scan", "media_path", s.mediaPath)

	err = filepath.WalkDir(s.mediaPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			s.logger.Error("walk error", "path", path, "error", walkErr)
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := supportedVideoExtensions[ext]; !ok {
			return nil
		}

		report.FilesFound++
		resolvedPath, err := filepath.Abs(path)
		if err != nil {
			s.logger.Error("resolve absolute path", "path", path, "error", err)
			report.Skipped++
			return nil
		}

		baseName, quality := extractQuality(d.Name(), ctx, resolvedPath, s.logger)
		video := ScannedVideo{
			Title:    titleFromFilename(baseName),
			Filename: d.Name(),
			Path:     resolvedPath,
			Duration: probeDurationSeconds(ctx, resolvedPath, s.logger),
			Quality:  quality,
		}
		groupedVideos[video.Title] = append(groupedVideos[video.Title], video)

		return nil
	})
	if err != nil {
		return report, fmt.Errorf("scan media library: %w", err)
	}

	for _, videos := range groupedVideos {
		for _, video := range videos {
			existing, err := s.store.GetVideoByPath(video.Path)
			switch {
			case err == nil:
				if err := s.store.UpdateVideoMetadata(existing.ID, video); err != nil {
					if s.store.IsUniqueFilenameError(err) {
						s.logger.Warn("skipping file due to filename conflict", "path", video.Path, "filename", video.Filename)
						report.Skipped++
						continue
					}
					return report, err
				}
				report.Updated++
				s.logger.Info("updated scanned video", "path", video.Path, "title", video.Title, "quality", video.Quality)
			case errors.Is(err, sql.ErrNoRows):
				if err := s.store.InsertVideo(video); err != nil {
					if s.store.IsUniqueFilenameError(err) {
						s.logger.Warn("skipping file due to filename conflict", "path", video.Path, "filename", video.Filename)
						report.Skipped++
						continue
					}
					return report, err
				}
				report.Inserted++
				s.logger.Info("added scanned video", "path", video.Path, "title", video.Title, "quality", video.Quality)
			default:
				return report, err
			}
		}
	}

	s.logger.Info("completed media scan", "files_found", report.FilesFound, "inserted", report.Inserted, "updated", report.Updated, "skipped", report.Skipped)
	return report, nil
}

func titleFromFilename(name string) string {
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ")
	base := replacer.Replace(name)
	fields := strings.Fields(base)
	for i, field := range fields {
		fields[i] = capitalizeWord(field)
	}
	return strings.Join(fields, " ")
}

func extractQuality(filename string, ctx context.Context, path string, logger *slog.Logger) (baseName, quality string) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	matches := qualityPattern.FindStringSubmatchIndex(base)
	if len(matches) >= 4 && matches[0] != -1 && matches[1] == len(base) {
		quality = normalizeQuality(base[matches[2]:matches[3]])
		baseName = strings.TrimSpace(base[:matches[0]])
		baseName = strings.TrimRight(baseName, "_- ")
		return baseName, quality
	}

	resolution := probeResolution(ctx, path, logger)
	if resolution != "" {
		return base, resolution
	}

	return base, ""
}

func normalizeQuality(quality string) string {
	if strings.EqualFold(quality, "4k") {
		return "4K"
	}
	return strings.ToLower(quality)
}

func capitalizeWord(word string) string {
	if word == "" {
		return ""
	}
	runes := []rune(strings.ToLower(word))
	runes[0] = unicode.ToTitle(runes[0])
	return string(runes)
}

func probeDurationSeconds(ctx context.Context, path string, logger *slog.Logger) int {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		probeCtx,
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		logger.Warn("ffprobe failed", "path", path, "error", err)
		return 0
	}

	value := strings.TrimSpace(string(output))
	if value == "" {
		return 0
	}

	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Warn("invalid ffprobe duration", "path", path, "value", value, "error", err)
		return 0
	}

	return int(seconds + 0.5)
}

func probeResolution(ctx context.Context, path string, logger *slog.Logger) string {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return ""
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		probeCtx,
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		logger.Warn("ffprobe resolution detection failed", "path", path, "error", err)
		return ""
	}

	resolution := strings.TrimSpace(string(output))
	if resolution == "" {
		return ""
	}

	parts := strings.Split(resolution, "x")
	if len(parts) != 2 {
		return ""
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		logger.Warn("invalid height value", "path", path, "value", parts[1], "error", err)
		return ""
	}

	switch {
	case height >= 2160:
		return "2160p"
	case height >= 1440:
		return "1440p"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 480:
		return "480p"
	case height >= 360:
		return "360p"
	default:
		return fmt.Sprintf("%dp", height)
	}
}
