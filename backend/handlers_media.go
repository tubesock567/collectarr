package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func (api *API) handleStreamVideo(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	file, err := os.Open(video.Path)
	if err != nil {
		api.logger.Error("open video failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "video file not found"})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		api.logger.Error("stat video failed", "id", video.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to stream video"})
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(video.Filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, video.Filename, info.ModTime(), file)
}

func (api *API) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	thumbnailPath, generated, err := api.ensureThumbnail(r.Context(), video)
	if err != nil {
		if errors.Is(err, errThumbnailFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "thumbnail generation requires ffmpeg"})
			return
		}

		api.logger.Error("ensure thumbnail failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to generate thumbnail"})
		return
	}

	if !generated {
		if _, err := os.Stat(thumbnailPath); err != nil {
			api.logger.Error("check thumbnail failed", "title", video.Title, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load thumbnail"})
			return
		}
	}

	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, thumbnailPath)
}

func (api *API) handlePreviewMetadata(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	scrubberVideo, err := api.store.GetVideoForScrubber(video.Title)
	if err != nil {
		api.logger.Error("get video for scrubber failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to get video for scrubber"})
		return
	}

	preview, err := api.ensurePreviewSprite(r.Context(), scrubberVideo)
	if err != nil {
		if errors.Is(err, errPreviewFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "preview generation requires ffmpeg"})
			return
		}

		api.logger.Error("ensure preview sprite failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to generate preview sprite"})
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

func (api *API) handlePreviewSprite(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	scrubberVideo, err := api.store.GetVideoForScrubber(video.Title)
	if err != nil {
		api.logger.Error("get video for scrubber failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to get video for scrubber"})
		return
	}

	_, err = api.ensurePreviewSprite(r.Context(), scrubberVideo)
	if err != nil {
		if errors.Is(err, errPreviewFFmpegMissing) {
			writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "preview generation requires ffmpeg"})
			return
		}

		api.logger.Error("load preview sprite failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load preview sprite"})
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, previewSpriteFilePath(video.Title))
}

func (api *API) handleHoverPreview(w http.ResponseWriter, r *http.Request) {
	video, err := api.videoFromRequest(r)
	if err != nil {
		api.writeVideoError(w, err)
		return
	}

	previewPath := hoverPreviewFilePath(video.Title)
	if _, err := os.Stat(previewPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "preview not found"})
			return
		}
		api.logger.Error("check hover preview failed", "title", video.Title, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load hover preview"})
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, previewPath)
}

func (api *API) handleGenerateThumbnails(w http.ResponseWriter, r *http.Request) {
	api.logger.Info("thumbnail generation enqueued")
	api.thumbWg.Add(1)
	go func() {
		defer api.thumbWg.Done()
		api.generateAllThumbnails()
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Thumbnail generation started in background",
	})
}

func (api *API) handleGeneratePreviews(w http.ResponseWriter, r *http.Request) {
	var req PreviewGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if !req.GenerateThumbnails && !req.GenerateScrubberSprites && !req.GenerateHoverPreviews {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "at least one preview type must be selected"})
		return
	}

	titles, err := api.store.ListAllTitles()
	if err != nil {
		api.logger.Error("failed to list titles for preview generation", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start preview generation"})
		return
	}

	status, started := api.beginPreviewGeneration("manual", req, len(titles))
	if !started {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "preview generation is already running"})
		return
	}

	if len(titles) == 0 {
		api.finishPreviewGeneration("completed", "No videos available for preview generation.")
		writeJSON(w, http.StatusOK, api.currentPreviewGenerationStatus())
		return
	}

	api.logger.Info("preview generation enqueued", "generate_thumbnails", req.GenerateThumbnails, "generate_scrubber_sprites", req.GenerateScrubberSprites, "generate_hover_previews", req.GenerateHoverPreviews)
	api.thumbWg.Add(1)
	go func() {
		defer api.thumbWg.Done()
		api.generatePreviews(context.Background(), req, titles)
	}()

	writeJSON(w, http.StatusAccepted, status)
}

func (api *API) handleGetPreviewGenerationStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.currentPreviewGenerationStatus())
}

func (api *API) generatePreviews(parent context.Context, req PreviewGenerationRequest, titles []string) {
	api.logger.Info("starting preview generation", "thumbnails", req.GenerateThumbnails, "scrubber_sprites", req.GenerateScrubberSprites, "hover_previews", req.GenerateHoverPreviews, "titles", len(titles))

	for _, title := range titles {
		if req.GenerateThumbnails {
			api.updatePreviewGenerationStep(title, "Generating thumbnails")
			video, err := api.store.GetVideoForThumbnail(title)
			if err != nil {
				api.logger.Error("get video for thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, _, err := api.ensureThumbnail(parent, video); err != nil {
				api.logger.Error("generate thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if req.GenerateScrubberSprites {
			api.updatePreviewGenerationStep(title, "Generating scrubber sprites")
			video, err := api.store.GetVideoForScrubber(title)
			if err != nil {
				api.logger.Error("get video for scrubber failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensurePreviewSprite(parent, video); err != nil {
				api.logger.Error("generate scrubber sprite failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if req.GenerateHoverPreviews {
			api.updatePreviewGenerationStep(title, "Generating hover previews")
			video, err := api.store.GetVideoForPreview(title)
			if err != nil {
				api.logger.Error("get video for preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensureHoverPreview(parent, video); err != nil {
				api.logger.Error("generate hover preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		api.completePreviewGenerationVideo()
	}

	api.logger.Info("preview generation complete", "titles", len(titles))
	status := api.currentPreviewGenerationStatus()
	message := "Preview generation complete."
	if status.Errors > 0 {
		message = fmt.Sprintf("Preview generation complete with %d errors.", status.Errors)
	}
	api.finishPreviewGeneration("completed", message)
}

func (api *API) generateConfiguredPreviewAssets(parent context.Context) {
	settings, err := api.store.GetGenerationSettings()
	if err != nil {
		api.logger.Error("load generation settings failed", "error", err)
		return
	}
	if !settings.AutoGenerateDuringScan {
		return
	}
	if !settings.GenerateThumbnails && !settings.GenerateScrubberSprites && !settings.GenerateHoverPreviews {
		return
	}

	titles, err := api.store.ListAllTitles()
	if err != nil {
		api.logger.Error("failed to list titles for preview generation", "error", err)
		return
	}

	req := PreviewGenerationRequest{
		GenerateThumbnails:      settings.GenerateThumbnails,
		GenerateScrubberSprites: settings.GenerateScrubberSprites,
		GenerateHoverPreviews:   settings.GenerateHoverPreviews,
	}

	if _, started := api.beginPreviewGeneration("scan", req, len(titles)); !started {
		api.logger.Info("skipping configured preview generation because another job is already running")
		return
	}

	if len(titles) == 0 {
		api.finishPreviewGeneration("completed", "No videos available for preview generation.")
		return
	}

	api.logger.Info("starting configured preview asset generation", "titles", len(titles), "thumbnails", settings.GenerateThumbnails, "scrubber_sprites", settings.GenerateScrubberSprites, "hover_previews", settings.GenerateHoverPreviews)
	for _, title := range titles {
		if settings.GenerateThumbnails {
			api.updatePreviewGenerationStep(title, "Generating thumbnails")
			video, err := api.store.GetVideoForThumbnail(title)
			if err != nil {
				api.logger.Error("get video for thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, _, err := api.ensureThumbnail(parent, video); err != nil {
				api.logger.Error("generate thumbnail failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if settings.GenerateScrubberSprites {
			api.updatePreviewGenerationStep(title, "Generating scrubber sprites")
			video, err := api.store.GetVideoForScrubber(title)
			if err != nil {
				api.logger.Error("get video for scrubber failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensurePreviewSprite(parent, video); err != nil {
				api.logger.Error("generate scrubber sprite failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		if settings.GenerateHoverPreviews {
			api.updatePreviewGenerationStep(title, "Generating hover previews")
			video, err := api.store.GetVideoForPreview(title)
			if err != nil {
				api.logger.Error("get video for preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			} else if _, err := api.ensureHoverPreview(parent, video); err != nil {
				api.logger.Error("generate hover preview failed", "title", title, "error", err)
				api.incrementPreviewGenerationErrors()
			}
		}
		api.completePreviewGenerationVideo()
	}
	api.logger.Info("configured preview asset generation complete", "titles", len(titles))
	status := api.currentPreviewGenerationStatus()
	message := "Preview generation complete."
	if status.Errors > 0 {
		message = fmt.Sprintf("Preview generation complete with %d errors.", status.Errors)
	}
	api.finishPreviewGeneration("completed", message)
}

func (api *API) enqueueConfiguredPreviewAssets() {
	api.logger.Info("configured preview generation enqueued")
	api.thumbWg.Add(1)
	go func() {
		defer api.thumbWg.Done()
		api.generateConfiguredPreviewAssets(context.Background())
	}()
}

func (api *API) beginPreviewGeneration(source string, req PreviewGenerationRequest, totalVideos int) (PreviewGenerationStatus, bool) {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	if api.previewStatus.Running {
		return api.previewStatus, false
	}

	now := time.Now()
	api.previewStatus = PreviewGenerationStatus{
		Status:                  "running",
		Source:                  source,
		Running:                 true,
		GenerateThumbnails:      req.GenerateThumbnails,
		GenerateScrubberSprites: req.GenerateScrubberSprites,
		GenerateHoverPreviews:   req.GenerateHoverPreviews,
		TotalVideos:             totalVideos,
		ProcessedVideos:         0,
		Errors:                  0,
		Message:                 "Preview generation started in background.",
		StartedAt:               &now,
		CompletedAt:             nil,
	}

	return api.previewStatus, true
}

func (api *API) updatePreviewGenerationStep(title string, step string) {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	api.previewStatus.CurrentVideoTitle = title
	api.previewStatus.CurrentStep = step
	api.previewStatus.Message = fmt.Sprintf("%s (%d/%d)", step, api.previewStatus.ProcessedVideos+1, api.previewStatus.TotalVideos)
}

func (api *API) completePreviewGenerationVideo() {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	api.previewStatus.ProcessedVideos++
	api.previewStatus.CurrentStep = ""
	api.previewStatus.CurrentVideoID = 0
	api.previewStatus.CurrentVideoTitle = ""
	if api.previewStatus.Running {
		api.previewStatus.Message = fmt.Sprintf("Processed %d of %d videos.", api.previewStatus.ProcessedVideos, api.previewStatus.TotalVideos)
	}
}

func (api *API) incrementPreviewGenerationErrors() {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	api.previewStatus.Errors++
}

func (api *API) finishPreviewGeneration(status string, message string) {
	api.previewMu.Lock()
	defer api.previewMu.Unlock()

	now := time.Now()
	api.previewStatus.Status = status
	api.previewStatus.Running = false
	api.previewStatus.CurrentStep = ""
	api.previewStatus.CurrentVideoID = 0
	api.previewStatus.CurrentVideoTitle = ""
	api.previewStatus.Message = message
	api.previewStatus.CompletedAt = &now
}

func (api *API) currentPreviewGenerationStatus() PreviewGenerationStatus {
	api.previewMu.RLock()
	defer api.previewMu.RUnlock()

	return api.previewStatus
}

func (api *API) generateAllThumbnails() {
	api.logger.Info("starting bulk thumbnail generation")

	titles, err := api.store.ListAllTitles()
	if err != nil {
		api.logger.Error("failed to list titles for thumbnails", "error", err)
		return
	}

	for i, title := range titles {
		api.logger.Info("generating thumbnail", "title", title, "progress", fmt.Sprintf("%d/%d", i+1, len(titles)))

		thumbnailPath := thumbnailFilePath(title)
		if _, err := os.Stat(thumbnailPath); err == nil {
			api.logger.Info("skipping cached thumbnail", "title", title)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			api.logger.Error("check cached thumbnail failed", "title", title, "error", err)
			continue
		}

		video, err := api.store.GetVideoForThumbnail(title)
		if err != nil {
			api.logger.Error("get video for thumbnail failed", "title", title, "error", err)
			continue
		}

		if _, _, err := api.ensureThumbnail(context.Background(), video); err != nil {
			if errors.Is(err, errThumbnailFFmpegMissing) {
				api.logger.Error("bulk thumbnail generation unavailable", "error", err)
				return
			}

			api.logger.Error("generate thumbnail in bulk failed", "title", title, "error", err)
		}
	}

	api.logger.Info("bulk thumbnail generation complete", "total", len(titles))
}

var errThumbnailFFmpegMissing = errors.New("ffmpeg not available")
var errPreviewFFmpegMissing = errors.New("ffmpeg not available for previews")

func (api *API) ensureThumbnail(parent context.Context, video Video) (string, bool, error) {
	thumbnailDir := thumbnailCacheDir()
	if err := os.MkdirAll(thumbnailDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create thumbnail directory: %w", err)
	}

	thumbnailPath := thumbnailFilePath(video.Title)
	if _, err := os.Stat(thumbnailPath); err == nil {
		api.logger.Info("thumbnail cache hit", "video_id", video.ID, "title", video.Title)
		return thumbnailPath, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("check thumbnail cache: %w", err)
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", false, errThumbnailFFmpegMissing
	}

	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	ssTime := "1"
	if video.Duration > 0 {
		minTime := float64(video.Duration) * 0.1
		maxTime := float64(video.Duration) * 0.9
		randomTime := minTime + api.randomFloat64()*(maxTime-minTime)
		ssTime = fmt.Sprintf("%.2f", randomTime)
	}

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-ss", ssTime,
		"-i", video.Path,
		"-frames:v", "1",
		"-vf", "scale=640:360",
		thumbnailPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("generate thumbnail: %w (output: %s)", err, string(output))
	}

	api.logger.Info("thumbnail generated", "video_id", video.ID, "title", video.Title, "path", thumbnailPath)
	return thumbnailPath, true, nil
}

func (api *API) ensurePlaylistCover(parent context.Context, playlistID int64) (string, error) {
	playlist, err := api.store.GetPlaylistByID(playlistID)
	if err != nil {
		return "", err
	}

	coverDir := playlistCoverCacheDir()
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		return "", fmt.Errorf("create playlist cover directory: %w", err)
	}

	coverPath := playlistCoverFilePath(playlistID)
	if coverInfo, err := os.Stat(coverPath); err == nil {
		if !playlistCoverNeedsRefresh(coverInfo, playlist) {
			return coverPath, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check playlist cover cache: %w", err)
	}

	thumbPaths := make([]string, 0, 4)
	for _, item := range playlist.Items {
		if len(thumbPaths) == 4 {
			break
		}
		thumbnailVideo, err := api.store.GetVideoForThumbnail(item.Title)
		if err != nil {
			continue
		}
		thumbnailPath, _, err := api.ensureThumbnail(parent, thumbnailVideo)
		if err != nil {
			return "", err
		}
		thumbPaths = append(thumbPaths, thumbnailPath)
	}

	if err := writePlaylistCoverImage(coverPath, thumbPaths); err != nil {
		return "", err
	}

	return coverPath, nil
}

func playlistCoverNeedsRefresh(coverInfo os.FileInfo, playlist Playlist) bool {
	if playlist.DateUpdated != nil && coverInfo.ModTime().Before(playlist.DateUpdated.UTC()) {
		return true
	}

	for i, item := range playlist.Items {
		if i == 4 {
			break
		}
		thumbnailPath := thumbnailFilePath(item.Title)
		thumbInfo, err := os.Stat(thumbnailPath)
		if err != nil {
			return true
		}
		if coverInfo.ModTime().Before(thumbInfo.ModTime()) {
			return true
		}
	}

	return false
}

func writePlaylistCoverImage(outputPath string, thumbPaths []string) error {
	const coverWidth = 640
	const coverHeight = 360
	const columns = 2
	const rows = 2
	cellWidth := coverWidth / columns
	cellHeight := coverHeight / rows

	canvas := image.NewRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	background := image.NewUniform(color.Gray{Y: 22})
	draw.Draw(canvas, canvas.Bounds(), background, image.Point{}, draw.Src)

	blank := image.NewUniform(color.Gray{Y: 36})
	for idx := 0; idx < columns*rows; idx++ {
		x := (idx % columns) * cellWidth
		y := (idx / columns) * cellHeight
		cellRect := image.Rect(x, y, x+cellWidth, y+cellHeight)
		draw.Draw(canvas, cellRect, blank, image.Point{}, draw.Src)
	}

	for idx, thumbPath := range thumbPaths {
		if idx == columns*rows {
			break
		}
		file, err := os.Open(thumbPath)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(file)
		_ = file.Close()
		if err != nil {
			continue
		}
		x := (idx % columns) * cellWidth
		y := (idx / columns) * cellHeight
		cellRect := image.Rect(x, y, x+cellWidth, y+cellHeight)
		draw.Draw(canvas, cellRect, resizeAndCropToRect(img, cellWidth, cellHeight), image.Point{}, draw.Src)
	}

	tempPath := outputPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create playlist cover file: %w", err)
	}
	if err := jpeg.Encode(file, canvas, &jpeg.Options{Quality: 85}); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("encode playlist cover: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close playlist cover file: %w", err)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace playlist cover file: %w", err)
	}

	return nil
}

func resizeAndCropToRect(src image.Image, width, height int) image.Image {
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth == 0 || srcHeight == 0 || width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	srcAspect := float64(srcWidth) / float64(srcHeight)
	dstAspect := float64(width) / float64(height)

	var cropWidth, cropHeight int
	var offsetX, offsetY int
	if srcAspect > dstAspect {
		cropHeight = srcHeight
		cropWidth = int(float64(cropHeight) * dstAspect)
		offsetX = (srcWidth - cropWidth) / 2
	} else {
		cropWidth = srcWidth
		cropHeight = int(float64(cropWidth) / dstAspect)
		offsetY = (srcHeight - cropHeight) / 2
	}
	if cropWidth <= 0 {
		cropWidth = srcWidth
	}
	if cropHeight <= 0 {
		cropHeight = srcHeight
	}

	for y := 0; y < height; y++ {
		srcY := offsetY + (y*cropHeight)/height
		for x := 0; x < width; x++ {
			srcX := offsetX + (x*cropWidth)/width
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	return dst
}

func (api *API) ensurePreviewSprite(parent context.Context, video Video) (PreviewSpriteResponse, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return PreviewSpriteResponse{}, errPreviewFFmpegMissing
	}

	previewDir := previewCacheDir()
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("create preview directory: %w", err)
	}

	spritePath := previewSpriteFilePath(video.Title)
	metadataPath := previewMetadataFilePath(video.Title)
	sourceInfo, err := os.Stat(video.Path)
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("stat source video: %w", err)
	}

	if spriteInfo, err := os.Stat(spritePath); err == nil {
		if metadata, err := readPreviewMetadata(metadataPath); err == nil && spriteInfo.ModTime().After(sourceInfo.ModTime()) {
			metadata.SpriteURL = fmt.Sprintf("/api/video/%d/preview-sprite", video.ID)
			api.logger.Info("preview sprite cache hit", "video_id", video.ID, "title", video.Title)
			return metadata, nil
		}
	}

	const frameWidth = 120
	const frameHeight = 68
	const columns = 8
	const sampleCount = 40
	rows := int(math.Ceil(float64(sampleCount) / float64(columns)))
	timestamps := api.samplePreviewTimestamps(video.Duration, sampleCount)

	tempDir, err := os.MkdirTemp(previewDir, fmt.Sprintf("video-%d-*", video.ID))
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("create preview temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	sprite := image.NewRGBA(image.Rect(0, 0, columns*frameWidth, rows*frameHeight))
	for idx, timestamp := range timestamps {
		framePath := filepath.Join(tempDir, fmt.Sprintf("frame-%02d.jpg", idx))
		if err := generatePreviewFrame(parent, ffmpegPath, video.Path, framePath, timestamp, frameWidth, frameHeight); err != nil {
			return PreviewSpriteResponse{}, err
		}

		frameFile, err := os.Open(framePath)
		if err != nil {
			return PreviewSpriteResponse{}, fmt.Errorf("open preview frame: %w", err)
		}
		frameImage, err := jpeg.Decode(frameFile)
		frameFile.Close()
		if err != nil {
			return PreviewSpriteResponse{}, fmt.Errorf("decode preview frame: %w", err)
		}

		col := idx % columns
		row := idx / columns
		draw.Draw(sprite, image.Rect(col*frameWidth, row*frameHeight, (col+1)*frameWidth, (row+1)*frameHeight), frameImage, image.Point{}, draw.Src)
	}

	spriteFile, err := os.Create(spritePath)
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("create preview sprite: %w", err)
	}
	if err := jpeg.Encode(spriteFile, sprite, &jpeg.Options{Quality: 85}); err != nil {
		spriteFile.Close()
		return PreviewSpriteResponse{}, fmt.Errorf("encode preview sprite: %w", err)
	}
	if err := spriteFile.Close(); err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("close preview sprite: %w", err)
	}

	metadata := PreviewSpriteResponse{
		SpriteURL:   fmt.Sprintf("/api/video/%d/preview-sprite", video.ID),
		FrameWidth:  frameWidth,
		FrameHeight: frameHeight,
		Columns:     columns,
		Rows:        rows,
		Timestamps:  timestamps,
		Duration:    video.Duration,
		SampleCount: sampleCount,
	}

	if err := writePreviewMetadata(metadataPath, metadata); err != nil {
		return PreviewSpriteResponse{}, err
	}

	api.logger.Info("preview sprite generated", "video_id", video.ID, "title", video.Title, "path", spritePath)
	return metadata, nil
}

func (api *API) ensureHoverPreview(parent context.Context, video Video) (string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", errPreviewFFmpegMissing
	}

	previewDir := hoverPreviewCacheDir()
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		return "", fmt.Errorf("create hover preview directory: %w", err)
	}

	previewPath := hoverPreviewFilePath(video.Title)
	sourceInfo, err := os.Stat(video.Path)
	if err != nil {
		return "", fmt.Errorf("stat source video: %w", err)
	}
	if previewInfo, err := os.Stat(previewPath); err == nil && previewInfo.ModTime().After(sourceInfo.ModTime()) {
		api.logger.Info("hover preview cache hit", "video_id", video.ID, "title", video.Title)
		return previewPath, nil
	}

	segmentCount := 12
	const segmentDuration = 1.5
	if video.Duration > 0 {
		segmentCount = max(1, min(segmentCount, int(math.Ceil(float64(video.Duration)/segmentDuration))))
	}
	timestamps := sampleHoverPreviewStartTimes(video.Duration, segmentCount, segmentDuration)
	tempDir, err := os.MkdirTemp(previewDir, fmt.Sprintf("hover-%d-*", video.ID))
	if err != nil {
		return "", fmt.Errorf("create hover preview temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	segmentPaths := make([]string, 0, len(timestamps))
	for idx, timestamp := range timestamps {
		segmentPath := filepath.Join(tempDir, fmt.Sprintf("segment-%02d.mp4", idx))
		if err := generateHoverPreviewSegment(parent, ffmpegPath, video.Path, segmentPath, timestamp, segmentDuration); err != nil {
			return "", err
		}
		segmentPaths = append(segmentPaths, segmentPath)
	}

	concatFile := filepath.Join(tempDir, "segments.txt")
	var builder strings.Builder
	for _, segmentPath := range segmentPaths {
		builder.WriteString("file '")
		builder.WriteString(strings.ReplaceAll(segmentPath, "'", "'\\''"))
		builder.WriteString("'\n")
	}
	if err := os.WriteFile(concatFile, []byte(builder.String()), 0o644); err != nil {
		return "", fmt.Errorf("write hover preview concat file: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		previewPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("generate hover preview: %w (output: %s)", err, string(output))
	}

	api.logger.Info("hover preview generated", "video_id", video.ID, "title", video.Title, "path", previewPath)
	return previewPath, nil
}

func thumbnailCacheDir() string {
	return "/data/previews/thumbnails"
}

func thumbnailFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(thumbnailCacheDir(), fmt.Sprintf("%s.jpg", safeTitle))
}

func previewCacheDir() string {
	return "/data/previews/scrubber-sprites"
}

func previewSpriteFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(previewCacheDir(), fmt.Sprintf("%s.jpg", safeTitle))
}

func previewMetadataFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(previewCacheDir(), fmt.Sprintf("%s.json", safeTitle))
}

func hoverPreviewCacheDir() string {
	return "/data/previews/hover-previews"
}

func hoverPreviewFilePath(title string) string {
	safeTitle := sanitizeFilename(title)
	return filepath.Join(hoverPreviewCacheDir(), fmt.Sprintf("%s.mp4", safeTitle))
}

func playlistCoverCacheDir() string {
	return "/data/previews/playlist-covers"
}

func playlistCoverFilePath(playlistID int64) string {
	return filepath.Join(playlistCoverCacheDir(), fmt.Sprintf("%d.jpg", playlistID))
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}

func generatePreviewFrame(parent context.Context, ffmpegPath, videoPath, outputPath string, timestamp float64, width, height int) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("generate preview frame: %w (output: %s)", err, string(output))
	}

	return nil
}

func generateHoverPreviewSegment(parent context.Context, ffmpegPath, videoPath, outputPath string, timestamp, segmentDuration float64) error {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-t", fmt.Sprintf("%.2f", segmentDuration),
		"-i", videoPath,
		"-an",
		"-vf", "scale=426:240",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("generate hover preview segment: %w (output: %s)", err, string(output))
	}
	return nil
}

func (api *API) samplePreviewTimestamps(duration, count int) []float64 {
	if count <= 0 {
		return nil
	}

	timestamps := make([]float64, 0, count)
	if duration <= 0 {
		for i := 0; i < count; i++ {
			timestamps = append(timestamps, float64(i+1))
		}
		return timestamps
	}

	minTime := math.Max(1, float64(duration)*0.1)
	maxTime := math.Max(minTime, float64(duration)*0.9)
	if count == 1 {
		return []float64{(minTime + maxTime) / 2}
	}
	step := (maxTime - minTime) / float64(count-1)
	for i := 0; i < count; i++ {
		timestamps = append(timestamps, minTime+(float64(i)*step))
	}
	return timestamps
}

func sampleHoverPreviewStartTimes(duration, count int, segmentDuration float64) []float64 {
	if count <= 0 {
		return nil
	}

	if duration <= 0 {
		starts := make([]float64, 0, count)
		for i := 0; i < count; i++ {
			starts = append(starts, float64(i)*segmentDuration)
		}
		return starts
	}

	maxStart := math.Max(0, float64(duration)-segmentDuration)
	if count == 1 {
		return []float64{maxStart / 2}
	}

	starts := make([]float64, 0, count)
	step := 0.0
	if count > 1 {
		step = maxStart / float64(count-1)
	}
	for i := 0; i < count; i++ {
		starts = append(starts, float64(i)*step)
	}
	return starts
}

func readPreviewMetadata(path string) (PreviewSpriteResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("read preview metadata: %w", err)
	}

	var metadata PreviewSpriteResponse
	if err := json.Unmarshal(data, &metadata); err != nil {
		return PreviewSpriteResponse{}, fmt.Errorf("decode preview metadata: %w", err)
	}

	return metadata, nil
}

func writePreviewMetadata(path string, metadata PreviewSpriteResponse) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode preview metadata: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write preview metadata: %w", err)
	}
	return nil
}

func (api *API) videoFromRequest(r *http.Request) (Video, error) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		return Video{}, err
	}
	return api.store.GetVideoByID(id)
}

func (api *API) writeVideoError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "video not found"})
		return
	}

	var numErr *strconv.NumError
	if errors.As(err, &numErr) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid video id"})
		return
	}

	api.logger.Error("video lookup failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load video"})
}
