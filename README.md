# Google Takeout EXIF Metadata Applier

A Go console application that processes Google Takeout data, extracting metadata from JSON sidecar files and applying it to media files (photos and videos).

## Features

- **Recursively scans** Google Takeout folders for JSON metadata files
- **Parses** Google Takeout JSON format containing:
  - Photo taken timestamp
  - GPS coordinates (latitude, longitude, altitude)
  - Image descriptions
  - View counts
- **Merges supplemental metadata** - Automatically combines data from `supplemental-metadata.json` files
- **Applies metadata** to media files:
  - Updates EXIF data for images (JPEG, PNG, GIF, HEIC, HEIF, etc.)
  - Updates file modification timestamps based on photo taken time
  - Supports both `geoData` and `geoDataAlt` GPS coordinates
- **Supported media formats:**
  - Images: JPG, JPEG, PNG, GIF, BMP, WebP, TIFF, HEIC, HEIF
  - Videos: MP4, AVI, MOV, MKV, FLV, WMV, WebM, M4V, 3GP, OGV, TS, MTS, M2TS
- **Dry-run mode** for testing without making changes
- **Verbose logging** for debugging

## Installation

### Prerequisites

- Go 1.21 or higher
- **FFmpeg** (optional, for video metadata support) - [Download FFmpeg](https://ffmpeg.org/download.html)
  - Windows: Add ffmpeg to your PATH or place `ffmpeg.exe` in the application directory
  - Linux: `sudo apt-get install ffmpeg` (Debian/Ubuntu) or `brew install ffmpeg` (macOS)

### Build from Source

```bash
cd google-takeout-exif-applier
go mod download
go build -o google-takeout-exif-applier.exe ./cmd
```

## Usage

```bash
google-takeout-exif-applier.exe -dir <path-to-takeout-folder> [options]
```

### Options

- `-dir string` - **Required** - Root directory of Google Takeout folder
- `-dry-run` - Perform a dry run without modifying files (optional)
- `-verbose` - Enable verbose logging to see detailed processing steps (optional)

### Examples

```bash
# Process a Google Takeout folder
google-takeout-exif-applier.exe -dir "C:\Takeout"

# Dry run to preview changes
google-takeout-exif-applier.exe -dir "C:\Takeout" -dry-run

# Process with verbose output
google-takeout-exif-applier.exe -dir "C:\Takeout" -verbose

# Combine options
google-takeout-exif-applier.exe -dir "C:\Takeout" -dry-run -verbose
```

## Google Takeout Structure

This tool expects the standard Google Takeout folder structure:

```
Takeout/
├── Google Photos/
│   ├── photo1.jpg
│   ├── photo1.jpg.json
│   ├── photo1.jpg-supplemental-metadata.json (optional)
│   ├── photo2.heic
│   ├── photo2.heic.json
│   ├── supplemental-metadata.json (optional, applies to all files)
│   └── ...
└── [other folders]
```

Each media file can have:
1. **Primary metadata file** (required): `filename.json` with standard metadata
2. **Supplemental metadata** (optional): `filename-supplemental-metadata.json` for additional data
3. **Global supplemental metadata** (optional): `supplemental-metadata.json` in folder applies to all files

## JSON Metadata Format

Example Google Takeout JSON metadata:

```json
{
  "title": "photo.jpg",
  "description": "A beautiful photo",
  "imageViews": "123",
  "creationTime": {
    "timestamp": "1609459200"
  },
  "photoTakenTime": {
    "timestamp": "1609459200"
  },
  "modificationTime": {
    "timestamp": "1609459200"
  },
  "geoData": {
    "latitude": 40.7128,
    "longitude": -74.0060,
    "altitude": 10.5,
    "latitudeSpan": 0.0,
    "longitudeSpan": 0.0
  }
}
```

## Metadata Applied

### For Images:
1. **Photo Taken Time** - Sets the EXIF DateTime
2. **GPS Coordinates** - Embeds latitude, longitude, and altitude in EXIF data
3. **Description** - Adds image description from metadata
4. **File Timestamps** - Updates file modification times

### For Videos:
1. **Creation Time** - Sets the creation_time metadata tag
2. **Title** - Sets the title metadata tag
3. **Description** - Adds comment metadata tag
4. **GPS Data** - Embeds GPS coordinates in comment field (requires ffmpeg)
5. **File Timestamps** - Updates file modification times

## Output

The application provides a summary report at the end:

```
=== Processing Complete ===
Total files scanned: 1500
JSON metadata files found: 750
Media files processed: 720
Files skipped: 30
Errors encountered: 0
```

## Advanced Features

- **Smart timestamp handling**: Falls back to creation time if photo taken time not available
- **Dual GPS data support**: Tries primary `geoData` then `geoDataAlt` if available
- **Error resilience**: Continues processing even if individual files fail
- **Selective processing**: Only processes supported media formats

## Limitations

- Video metadata application requires FFmpeg to be installed on your system
- If FFmpeg is not available, videos will fall back to timestamp-only updates
- Current EXIF implementation primarily handles JPEG embedding; other image formats use timestamp updates
- GPS data in videos is embedded as comment text (full GPS track support would require advanced tools)

## Future Enhancements

- [ ] Full video metadata support via ffmpeg integration
- [ ] Batch processing with progress bar
- [ ] Configuration file support
- [ ] Generate detailed processing report (CSV)
- [ ] Undo/rollback functionality
- [ ] Parallel processing for large folders
- [ ] Subtitle/chapter metadata support

## Troubleshooting

### "No matching media file" warnings
This is normal if there are JSON files without corresponding media files in your Takeout.

### File permission errors
Ensure you have write permissions to the media files you want to process.

### Skipped files
Check the verbose output (`-verbose` flag) to see why specific files were skipped.

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
