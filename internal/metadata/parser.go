package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Metadata represents the parsed metadata from Google Takeout JSON files
type Metadata struct {
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	ImageViews       int64            `json:"imageViews,string"`
	CreationTime     CreationTime     `json:"creationTime"`
	ModificationTime ModificationTime `json:"modificationTime"`
	GeoData          GeoData          `json:"geoData"`
	GeoDataAlt       GeoDataAlt       `json:"geoDataAlt"`
	PhotoTakenTime   PhotoTakenTime   `json:"photoTakenTime"`
	Supplemental     *Metadata        `json:"supplemental,omitempty"`
}

// CreationTime represents the creation timestamp
type CreationTime struct {
	Timestamp string `json:"timestamp"`
}

// ModificationTime represents the modification timestamp
type ModificationTime struct {
	Timestamp string `json:"timestamp"`
}

// PhotoTakenTime represents when the photo was taken
type PhotoTakenTime struct {
	Timestamp string `json:"timestamp"`
}

// GeoData represents GPS coordinates
type GeoData struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Altitude      float64 `json:"altitude"`
	LatitudeSpan  float64 `json:"latitudeSpan"`
	LongitudeSpan float64 `json:"longitudeSpan"`
}

// GeoDataAlt represents alternative GPS coordinates
type GeoDataAlt struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Altitude      float64 `json:"altitude"`
	LatitudeSpan  float64 `json:"latitudeSpan"`
	LongitudeSpan float64 `json:"longitudeSpan"`
}

// ParseJSON parses the metadata from a Google Takeout JSON file
func ParseJSON(jsonPath string) (*Metadata, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Remove UTF-8 BOM if present
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	var meta Metadata
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Check for supplemental metadata in the same directory
	baseDir := filepath.Dir(jsonPath)
	fileName := filepath.Base(jsonPath)

	// Remove the .json extension to get the media filename
	mediaFileName := strings.TrimSuffix(fileName, ".json")

	// Try to find supplemental metadata files
	// Google Takeout has:
	// 1. Primary metadata: photo.jpg.json
	// 2. Per-file supplemental: photo.jpg.supplemental-metadata.json
	// 3. Global supplemental: supplemental-metadata.json

	// First try exact supplemental-metadata.json in same directory (global)
	supplementalPath := filepath.Join(baseDir, "supplemental-metadata.json")
	if _, err := os.Stat(supplementalPath); err == nil {
		supplemental, err := parseSupplementalJSON(supplementalPath)
		if err == nil {
			meta = mergeMetadata(meta, *supplemental)
		}
	}

	// Then try per-file supplemental metadata: [mediafile].supplemental-metadata.json
	perFileSupplementalPath := filepath.Join(baseDir, mediaFileName+".supplemental-metadata.json")
	if _, err := os.Stat(perFileSupplementalPath); err == nil {
		supplemental, err := parseSupplementalJSON(perFileSupplementalPath)
		if err == nil {
			meta = mergeMetadata(meta, *supplemental)
		}
	}

	return &meta, nil
}

// parseSupplementalJSON parses supplemental metadata, avoiding infinite recursion
func parseSupplementalJSON(jsonPath string) (*Metadata, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Remove UTF-8 BOM if present
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	var meta Metadata
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &meta, nil
}

// GetPhotoTime returns the photo taken time, or creation time as fallback
func (m *Metadata) GetPhotoTime() (time.Time, error) {
	if m.PhotoTakenTime.Timestamp != "" {
		return parseTimestamp(m.PhotoTakenTime.Timestamp)
	}

	if m.CreationTime.Timestamp != "" {
		return parseTimestamp(m.CreationTime.Timestamp)
	}

	return time.Time{}, fmt.Errorf("no valid timestamp found in metadata")
}

// GetLatitude returns the latitude, preferring geoData over geoDataAlt
func (m *Metadata) GetLatitude() (float64, bool) {
	if m.GeoData.Latitude != 0 {
		return m.GeoData.Latitude, true
	}
	if m.GeoDataAlt.Latitude != 0 {
		return m.GeoDataAlt.Latitude, true
	}
	return 0, false
}

// GetLongitude returns the longitude, preferring geoData over geoDataAlt
func (m *Metadata) GetLongitude() (float64, bool) {
	if m.GeoData.Longitude != 0 {
		return m.GeoData.Longitude, true
	}
	if m.GeoDataAlt.Longitude != 0 {
		return m.GeoDataAlt.Longitude, true
	}
	return 0, false
}

// GetAltitude returns the altitude, preferring geoData over geoDataAlt
func (m *Metadata) GetAltitude() (float64, bool) {
	if m.GeoData.Altitude != 0 {
		return m.GeoData.Altitude, true
	}
	if m.GeoDataAlt.Altitude != 0 {
		return m.GeoDataAlt.Altitude, true
	}
	return 0, false
}

// mergeMetadata merges supplemental metadata into primary metadata
func mergeMetadata(primary, supplemental Metadata) Metadata {
	// Prefer primary over supplemental for most fields, but use supplemental if primary is empty
	if primary.Title == "" && supplemental.Title != "" {
		primary.Title = supplemental.Title
	}
	if primary.Description == "" && supplemental.Description != "" {
		primary.Description = supplemental.Description
	}
	if primary.ImageViews == 0 && supplemental.ImageViews > 0 {
		primary.ImageViews = supplemental.ImageViews
	}
	// Use supplemental creation time if primary doesn't have it
	if primary.CreationTime.Timestamp == "" && supplemental.CreationTime.Timestamp != "" {
		primary.CreationTime = supplemental.CreationTime
	}
	// Use supplemental photo taken time if primary doesn't have it
	if primary.PhotoTakenTime.Timestamp == "" && supplemental.PhotoTakenTime.Timestamp != "" {
		primary.PhotoTakenTime = supplemental.PhotoTakenTime
	}
	// Prefer primary geo data, fall back to supplemental
	if (primary.GeoData.Latitude == 0 && primary.GeoData.Longitude == 0) &&
		(supplemental.GeoData.Latitude != 0 || supplemental.GeoData.Longitude != 0) {
		primary.GeoData = supplemental.GeoData
	}
	if (primary.GeoDataAlt.Latitude == 0 && primary.GeoDataAlt.Longitude == 0) &&
		(supplemental.GeoDataAlt.Latitude != 0 || supplemental.GeoDataAlt.Longitude != 0) {
		primary.GeoDataAlt = supplemental.GeoDataAlt
	}
	return primary
}

// parseTimestamp converts a timestamp string to time.Time
func parseTimestamp(ts string) (time.Time, error) {
	// Google Takeout uses Unix timestamp as string
	var unixTime int64
	_, err := fmt.Sscanf(ts, "%d", &unixTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp format: %s", ts)
	}

	return time.Unix(unixTime, 0).UTC(), nil
}
