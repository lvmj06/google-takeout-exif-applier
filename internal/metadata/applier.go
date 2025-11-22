package metadata

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ApplyResult indicates whether a file was modified and provides details
type ApplyResult struct {
	Modified     bool
	Details      string
	ExistingData string
	NewData      string
}

// ApplyToFile applies the metadata to a media file
func ApplyToFile(mediaPath string, meta *Metadata) (*ApplyResult, error) {
	// Check if it's an image
	if isImageFile(mediaPath) {
		return applyToImage(mediaPath, meta)
	}

	// Check if it's a video
	if isVideoFile(mediaPath) {
		return applyToVideo(mediaPath, meta)
	}

	return nil, fmt.Errorf("unsupported media file type: %s", filepath.Ext(mediaPath))
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	imageExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".bmp":  true,
		".webp": true,
		".tiff": true,
		".tif":  true,
		".heic": true,
		".heif": true,
	}
	return imageExts[ext]
}

func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := map[string]bool{
		".mp4":  true,
		".avi":  true,
		".mov":  true,
		".mkv":  true,
		".flv":  true,
		".wmv":  true,
		".webm": true,
		".m4v":  true,
		".3gp":  true,
		".ogv":  true,
		".ts":   true,
		".mts":  true,
		".m2ts": true,
	}
	return videoExts[ext]
}

// applyToImage applies metadata to image files using exiftool if available, otherwise just timestamps
func applyToImage(imagePath string, meta *Metadata) (*ApplyResult, error) {
	photoTime, err := meta.GetPhotoTime()
	if err != nil {
		return nil, fmt.Errorf("no valid timestamp in metadata: %w", err)
	}

	result := &ApplyResult{
		Details:  filepath.Base(imagePath),
		Modified: false,
	}

	// Try using exiftool first if available
	_, err = exec.LookPath("exiftool")
	if err == nil {
		return applyImageMetadataWithExiftool(imagePath, meta, photoTime, result)
	}

	// Fallback to just updating timestamps if exiftool not available
	fmt.Printf("[INFO] exiftool not found, updating timestamps only for: %s\n", imagePath)
	err = os.Chtimes(imagePath, photoTime, photoTime)
	if err != nil {
		return result, fmt.Errorf("failed to update file times: %w", err)
	}

	result.Modified = true
	result.NewData = fmt.Sprintf("DateTime=%s", photoTime.Format("2006-01-02 15:04:05"))
	return result, nil
}

// applyImageMetadataWithExiftool uses exiftool to embed metadata and check existing data
func applyImageMetadataWithExiftool(imagePath string, meta *Metadata, photoTime time.Time, result *ApplyResult) (*ApplyResult, error) {
	// First, check existing EXIF data
	existingData := getExistingImageEXIF(imagePath)
	result.ExistingData = existingData

	// Prepare new metadata to check against existing
	newDateTime := photoTime.Format("2006:01:02 15:04:05")

	// Check if EXIF already matches what we want to write
	if existingData != "" && shouldSkipImageModification(existingData, newDateTime, meta) {
		result.Modified = false
		result.NewData = fmt.Sprintf("DateTime=%s", photoTime.Format("2006-01-02 15:04:05"))
		return result, nil
	}

	// EXIF data needs updating, proceed with exiftool
	args := []string{
		"-overwrite_original",
		fmt.Sprintf("-DateTime=%s", newDateTime),
	}

	// Add description if available
	if meta.Description != "" {
		args = append(args, fmt.Sprintf("-ImageDescription=%s", meta.Description))
		args = append(args, fmt.Sprintf("-Comment=%s", meta.Description))
	}

	// Add GPS data if available
	if lat, latOk := meta.GetLatitude(); latOk {
		if lon, lonOk := meta.GetLongitude(); lonOk {
			args = append(args, fmt.Sprintf("-GPSLatitude=%f", lat))
			args = append(args, fmt.Sprintf("-GPSLongitude=%f", lon))

			if alt, altOk := meta.GetAltitude(); altOk {
				args = append(args, fmt.Sprintf("-GPSAltitude=%f", alt))
			}
		}
	}

	args = append(args, imagePath)

	cmd := exec.Command("exiftool", args...)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("[WARN] exiftool failed, updating timestamps only: %v\n", err)
		// Fall back to timestamps
		err = os.Chtimes(imagePath, photoTime, photoTime)
		if err != nil {
			return result, fmt.Errorf("failed to update file times: %w", err)
		}
		result.Modified = true
		result.NewData = fmt.Sprintf("DateTime=%s", photoTime.Format("2006-01-02 15:04:05"))
		return result, nil
	}

	// Update file modification time
	err = os.Chtimes(imagePath, photoTime, photoTime)
	if err != nil {
		return result, fmt.Errorf("failed to update file times: %w", err)
	}

	result.Modified = true
	gpsStr := fmt.Sprintf("DateTime=%s", photoTime.Format("2006-01-02 15:04:05"))
	if lat, latOk := meta.GetLatitude(); latOk {
		if lon, lonOk := meta.GetLongitude(); lonOk {
			gpsStr = fmt.Sprintf("%s, GPS: %.6f, %.6f", gpsStr, lat, lon)
		}
	}
	result.NewData = gpsStr
	return result, nil
}

// shouldSkipImageModification checks if the existing EXIF data matches what we want
func shouldSkipImageModification(existingData string, newDateTime string, meta *Metadata) bool {
	// If no existing data, we need to modify
	if strings.TrimSpace(existingData) == "" {
		return false
	}

	// Check if DateTime matches (allow some flexibility in format)
	existingLower := strings.ToLower(existingData)

	// Simple check: if the new datetime appears in existing data, likely already set
	if strings.Contains(existingLower, strings.ToLower(strings.Split(newDateTime, " ")[0])) {
		return true
	}

	return false
}

// getExistingImageEXIF retrieves existing EXIF data from an image
func getExistingImageEXIF(imagePath string) string {
	cmd := exec.Command("exiftool", "-DateTime", "-GPSLatitude", "-GPSLongitude", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// applyToVideo applies metadata to video files using ffmpeg
func applyToVideo(videoPath string, meta *Metadata) (*ApplyResult, error) {
	result := &ApplyResult{
		Details:  filepath.Base(videoPath),
		Modified: false,
	}

	// Check if ffmpeg is available
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		// Fallback to just updating timestamps if ffmpeg is not available
		fmt.Printf("[WARN] ffmpeg not found, updating timestamps only for: %s\n", videoPath)
		photoTime, err := meta.GetPhotoTime()
		if err == nil {
			result.Modified = true
			result.NewData = fmt.Sprintf("DateTime=%s", photoTime.Format("2006-01-02 15:04:05"))
			return result, os.Chtimes(videoPath, photoTime, photoTime)
		}
		return result, fmt.Errorf("ffmpeg not available and no valid timestamp in metadata")
	}

	photoTime, err := meta.GetPhotoTime()
	if err != nil {
		return result, fmt.Errorf("no valid timestamp in metadata: %w", err)
	}

	// Create a temporary output file
	tempOutput := videoPath + ".tmp"
	defer func() {
		os.Remove(tempOutput)
	}()

	// Build ffmpeg command to add metadata
	args := []string{
		"-i", videoPath,
		"-metadata", fmt.Sprintf("creation_time=%s", photoTime.Format("2006-01-02T15:04:05")),
		"-metadata", fmt.Sprintf("title=%s", meta.Title),
	}

	// Add description as comment if available
	if meta.Description != "" {
		args = append(args, "-metadata", fmt.Sprintf("comment=%s", meta.Description))
	}

	// Add GPS metadata if available
	if lat, latOk := meta.GetLatitude(); latOk {
		if lon, lonOk := meta.GetLongitude(); lonOk {
			gpsStr := fmt.Sprintf("GPS: %.6f, %.6f", lat, lon)
			if alt, altOk := meta.GetAltitude(); altOk {
				gpsStr = fmt.Sprintf("%s, %.1fm", gpsStr, alt)
			}
			args = append(args, "-metadata", fmt.Sprintf("location=%s", gpsStr))
		}
	}

	// Add codec and output file
	args = append(args, "-c", "copy", "-y", tempOutput)

	cmd := exec.Command("ffmpeg", args...)

	// Run ffmpeg
	err = cmd.Run()
	if err != nil {
		return result, fmt.Errorf("ffmpeg failed: %w", err)
	}

	// Replace original with temp file
	err = os.Rename(tempOutput, videoPath)
	if err != nil {
		return result, fmt.Errorf("failed to replace original video: %w", err)
	}

	// Update file modification time
	err = os.Chtimes(videoPath, photoTime, photoTime)
	if err != nil {
		return result, fmt.Errorf("failed to update file times: %w", err)
	}

	result.Modified = true
	result.NewData = fmt.Sprintf("creation_time=%s", photoTime.Format("2006-01-02T15:04:05"))
	return result, nil
}
