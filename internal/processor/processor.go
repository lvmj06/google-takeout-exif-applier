package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"google-takeout-exif-applier/internal/metadata"
)

type Statistics struct {
	TotalFiles        int
	JSONFiles         int
	ProcessedFiles    int
	ModifiedFiles     int
	UnmodifiedFiles   int
	SkippedFiles      int
	ErrorCount        int
	ModifiedDetails   []string
	UnmodifiedDetails []string
	mu                sync.Mutex // Protect concurrent access to stats
}

type Processor struct {
	rootDir      string
	dryRun       bool
	verbose      bool
	stats        Statistics
	deletedFiles map[string]bool // Track deleted supplemental files
	deletedMutex sync.Mutex      // Protect deletedFiles map
	workerCount  int             // Number of concurrent workers
}

type fileJob struct {
	mediaPath string
}

type processResult struct {
	processed bool
	jobData   fileJob
}

func New(rootDir string, dryRun, verbose bool) *Processor {
	// Default to number of CPUs for worker count, but at least 2
	workerCount := runtime.NumCPU()
	if workerCount < 2 {
		workerCount = 2
	}

	return &Processor{
		rootDir:      rootDir,
		dryRun:       dryRun,
		verbose:      verbose,
		workerCount:  workerCount,
		deletedFiles: make(map[string]bool),
	}
}

func (p *Processor) Process() (Statistics, error) {
	// Create channels for worker pool
	jobChan := make(chan fileJob, p.workerCount*2)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go p.processWorker(&wg, jobChan)
	}

	// Collect media files to process
	var filesToProcess []string
	err := filepath.Walk(p.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If it's a "file not found" error for a supplemental metadata file, just skip it
			// (it may have been deleted during processing)
			if os.IsNotExist(err) && strings.HasSuffix(path, ".supplemental-metadata.json") {
				return nil
			}
			return err
		}

		if info.IsDir() {
			return nil
		}

		p.stats.mu.Lock()
		p.stats.TotalFiles++
		p.stats.mu.Unlock()

		// Skip supplemental metadata files - these are handled as part of media file processing
		p.deletedMutex.Lock()
		deleted := p.deletedFiles[path]
		p.deletedMutex.Unlock()

		if strings.HasSuffix(path, ".supplemental-metadata.json") || deleted {
			return nil
		}

		// Check if it's a supported media file (not JSON, not supplemental)
		if isSupportedMediaFile(path) {
			if p.verbose {
				fmt.Printf("[MEDIA] Found media file: %s\n", path)
			}
			filesToProcess = append(filesToProcess, path)
		}

		return nil
	})

	if err != nil {
		p.stats.mu.Lock()
		p.stats.ErrorCount++
		p.stats.mu.Unlock()
		close(jobChan)
		wg.Wait()
		return p.getStatsCopy(), fmt.Errorf("error walking directory: %w", err)
	}

	// Send jobs to workers
	go func() {
		for _, mediaPath := range filesToProcess {
			jobChan <- fileJob{mediaPath: mediaPath}
		}
		close(jobChan)
	}()

	// Wait for all workers to complete
	wg.Wait()

	return p.getStatsCopy(), nil
}

// getStatsCopy returns a copy of statistics without the mutex
func (p *Processor) getStatsCopy() Statistics {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()
	return Statistics{
		TotalFiles:        p.stats.TotalFiles,
		JSONFiles:         p.stats.JSONFiles,
		ProcessedFiles:    p.stats.ProcessedFiles,
		ModifiedFiles:     p.stats.ModifiedFiles,
		UnmodifiedFiles:   p.stats.UnmodifiedFiles,
		SkippedFiles:      p.stats.SkippedFiles,
		ErrorCount:        p.stats.ErrorCount,
		ModifiedDetails:   p.stats.ModifiedDetails,
		UnmodifiedDetails: p.stats.UnmodifiedDetails,
	}
}

// processWorker processes media files from the job channel
func (p *Processor) processWorker(wg *sync.WaitGroup, jobChan chan fileJob) {
	defer wg.Done()
	for job := range jobChan {
		p.processMediaFile(job.mediaPath)
	}
}

func (p *Processor) checkSupplementalData(mediaPath string) (os.FileInfo, string, error) {
	var info os.FileInfo
	var err error
	var jsonPath string
	var newMediaPath string

	// Check first if json will be found by replacing the file extension with json.
	ext := filepath.Ext(mediaPath)
	newMediaPath = mediaPath
	jsonPath = strings.TrimSuffix(mediaPath, ext) + ".json"

	info, err = os.Stat(jsonPath)

	if err == nil {
		return info, jsonPath, err
	}

	// Check if media path has (1), (2) suffixes before extension.
	// Compile the regex once for efficiency
	//var re = regexp.MustCompile(`\((\d+)\)(\.[^.]+)?$`)
	var re = regexp.MustCompile(`\(\d+\)`)
	var matches []string
	var match string

	// FindStringSubmatch returns a slice of strings:
	// [full_match, captured_group_1, captured_group_2, ...]
	// In our regex, captured_group_1 will be the '1' or '2'
	idx := strings.LastIndex(mediaPath, `\`)
	if idx != -1 {
		matches = re.FindAllString(mediaPath[idx+1:], -1)
	}

	if len(matches) == 1 {
		match = matches[0]
		fileName := strings.Replace(mediaPath[idx+1:], match, "", 1)
		newMediaPath = mediaPath[:idx+1] + fileName
	} else {
		newMediaPath = mediaPath
	}

	jsonSuffixes := [...]string{
		"",
		".supplemental-metadata",
		".supplemental-metadat",
		".supplemental-metada",
		".supplemental-metad",
		".supplemental-meta",
		".supplemental-met",
		".supplemental-me",
		".supplemental-m",
		".supplemental-",
		".supplemental",
		".supplementa",
		".supplement",
		".supplemen",
		".suppleme",
		".supplem",
		".supple",
		".suppl",
		".supp",
		".sup",
		".su",
		".s",
	}

	for _, suffix := range jsonSuffixes {
		// Look for supplemental metadata file: [mediafile].supplemental-metadata.json
		if match != "" {
			suffix = suffix + match + ".json"
		} else {
			suffix = suffix + ".json"
		}

		jsonPath = newMediaPath + suffix

		// Check if metadata exists
		info, err = os.Stat(jsonPath)

		if err == nil {
			return info, jsonPath, err
		}
	}

	// Reset newMediaPath
	newMediaPath = mediaPath

	// Another check for files with (1), (2)... but are not duplicates
	for _, suffix := range jsonSuffixes {

		// Look for supplemental metadata file: [mediafile].supplemental-metadata.json
		suffix = suffix + ".json"

		jsonPath = newMediaPath + suffix

		// Check if metadata exists
		info, err = os.Stat(jsonPath)

		if err == nil {
			return info, jsonPath, err
		}
	}

	return info, jsonPath, err
}

func (p *Processor) processMediaFile(mediaPath string) bool {
	// Look for supplemental metadata file: [mediafile].supplemental-metadata.json
	info, jsonPath, err := p.checkSupplementalData(mediaPath)

	if err != nil {
		if os.IsNotExist(err) {
			if p.verbose {
				fmt.Printf("[SKIP] No metadata file for: %s\n", mediaPath)
			}
		} else {
			p.stats.mu.Lock()
			p.stats.ErrorCount++
			p.stats.mu.Unlock()
			fmt.Printf("[ERROR] Cannot access metadata file %s: %v\n", jsonPath, err)
		}
		return false
	}

	if info.IsDir() {
		if p.verbose {
			fmt.Printf("[SKIP] Metadata path is a directory: %s\n", jsonPath)
		}
		return false
	}

	// Parse metadata from JSON - it will automatically find supplemental files
	meta, err := metadata.ParseJSON(jsonPath)
	if err != nil {
		p.stats.mu.Lock()
		p.stats.ErrorCount++
		p.stats.mu.Unlock()
		fmt.Printf("[ERROR] Failed to parse metadata from %s: %v\n", jsonPath, err)
		return false
	}

	p.stats.mu.Lock()
	p.stats.JSONFiles++
	p.stats.mu.Unlock()

	// Apply metadata to media file
	if p.dryRun {
		fmt.Printf("[DRY-RUN] Would apply metadata to: %s\n", mediaPath)
		if p.verbose {
			fmt.Printf("          Metadata: %+v\n", meta)
			fmt.Printf("          Would delete: %s\n", jsonPath)
		}
		p.stats.mu.Lock()
		p.stats.ProcessedFiles++
		p.stats.ModifiedFiles++
		detail := fmt.Sprintf("  %s (would be modified)", filepath.Base(mediaPath))
		p.stats.ModifiedDetails = append(p.stats.ModifiedDetails, detail)
		p.stats.mu.Unlock()
		return true
	}

	result, err := metadata.ApplyToFile(mediaPath, meta)
	if err != nil {
		p.stats.mu.Lock()
		p.stats.ErrorCount++
		p.stats.mu.Unlock()
		fmt.Printf("[ERROR] Failed to apply metadata to %s: %v\n", mediaPath, err)
		return false
	}

	p.stats.mu.Lock()
	p.stats.ProcessedFiles++
	if result.Modified {
		p.stats.ModifiedFiles++
		detail := fmt.Sprintf("  %s", result.Details)
		if result.NewData != "" {
			detail = fmt.Sprintf("%s\n    Modified: %s", detail, result.NewData)
		}
		p.stats.ModifiedDetails = append(p.stats.ModifiedDetails, detail)
		fmt.Printf("[OK] Metadata modified: %s\n", mediaPath)
		if p.verbose && result.ExistingData != "" {
			fmt.Printf("    Previous: %s\n", result.ExistingData)
			fmt.Printf("    Updated:  %s\n", result.NewData)
		}
	} else {
		p.stats.UnmodifiedFiles++
		detail := fmt.Sprintf("  %s", result.Details)
		if result.ExistingData != "" {
			detail = fmt.Sprintf("%s\n    Verified: %s", detail, result.ExistingData)
		}
		p.stats.UnmodifiedDetails = append(p.stats.UnmodifiedDetails, detail)
		fmt.Printf("[SKIP] Already up-to-date: %s\n", mediaPath)
		if p.verbose && result.ExistingData != "" {
			fmt.Printf("    Verified: %s\n", result.ExistingData)
		}
	}
	p.stats.mu.Unlock()

	// Delete supplemental metadata file after successful processing
	err = os.Remove(jsonPath)
	if err != nil {
		fmt.Printf("[WARN] Failed to delete supplemental metadata file %s: %v\n", jsonPath, err)
	} else {
		p.deletedMutex.Lock()
		p.deletedFiles[jsonPath] = true // Mark as deleted to skip if encountered in walk
		p.deletedMutex.Unlock()
		if p.verbose {
			fmt.Printf("    Deleted: %s\n", jsonPath)
		}
	}

	return true
}

func isSupportedMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Image formats
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
		".dng":  true,
	}

	// Video formats
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

	if imageExts[ext] || videoExts[ext] {
		return true
	}

	return false
}
