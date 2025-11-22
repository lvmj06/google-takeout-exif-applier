@echo off
REM Build script for Google Takeout EXIF Applier

echo Building Google Takeout EXIF Applier...
echo.

REM Download dependencies
echo Downloading dependencies...
go mod download

if errorlevel 1 (
    echo Error downloading dependencies
    exit /b 1
)

REM Build the application
echo Building executable...
go build -o google-takeout-exif-applier.exe ./cmd

if errorlevel 1 (
    echo Error building application
    exit /b 1
)

echo.
echo Build complete! Executable created: google-takeout-exif-applier.exe
echo.
echo Usage:
echo   google-takeout-exif-applier.exe -dir "C:\path\to\takeout" [options]
echo.
echo Options:
echo   -dry-run     Perform a dry run without modifying files
echo   -verbose     Enable verbose logging
