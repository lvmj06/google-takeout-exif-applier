package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"google-takeout-exif-applier/internal/processor"
)

func main() {
	rootDir := flag.String("dir", "", "Root directory of Google Takeout folder")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without modifying files")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	if *rootDir == "" {
		fmt.Println("Usage: google-takeout-exif-applier -dir <path-to-takeout-folder> [options]")
		fmt.Println("\nOptions:")
		fmt.Println("  -dir string      Root directory of Google Takeout folder (required)")
		fmt.Println("  -dry-run         Perform a dry run without modifying files")
		fmt.Println("  -verbose         Enable verbose logging")
		os.Exit(1)
	}

	// Verify directory exists
	info, err := os.Stat(*rootDir)
	if err != nil {
		log.Fatalf("Error accessing directory: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("Path is not a directory: %s", *rootDir)
	}

	absDir, err := filepath.Abs(*rootDir)
	if err != nil {
		log.Fatalf("Error getting absolute path: %v", err)
	}

	fmt.Printf("Starting Google Takeout EXIF metadata processor\n")
	fmt.Printf("Directory: %s\n", absDir)
	fmt.Printf("Dry Run: %v\n", *dryRun)
	fmt.Printf("Verbose: %v\n\n", *verbose)

	p := processor.New(absDir, *dryRun, *verbose)
	stats, err := p.Process()
	if err != nil {
		log.Fatalf("Error processing folder: %v", err)
	}

	fmt.Println("\n=== Processing Complete ===")
	fmt.Printf("Total files scanned: %d\n", stats.TotalFiles)
	fmt.Printf("JSON metadata files found: %d\n", stats.JSONFiles)
	fmt.Printf("Media files processed: %d\n", stats.ProcessedFiles)
	fmt.Printf("  - Modified: %d\n", stats.ModifiedFiles)
	fmt.Printf("  - Already up-to-date: %d\n", stats.UnmodifiedFiles)
	fmt.Printf("Files skipped: %d\n", stats.SkippedFiles)
	fmt.Printf("Errors encountered: %d\n", stats.ErrorCount)

	if *verbose && len(stats.ModifiedDetails) > 0 {
		fmt.Println("\n=== Modified Files ===")
		for _, detail := range stats.ModifiedDetails {
			fmt.Printf("%s\n", detail)
		}
	}

	if *verbose && len(stats.UnmodifiedDetails) > 0 {
		fmt.Println("\n=== Unchanged Files (Already Had Matching EXIF) ===")
		for _, detail := range stats.UnmodifiedDetails {
			fmt.Printf("%s\n", detail)
		}
	}

	if stats.ErrorCount > 0 {
		os.Exit(1)
	}
}
