// File: internal/logger/rotation.go
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	MaxLogFileSize = 10 * 1024 * 1024 // 10MB
	MaxLogAge      = 5 * 24 * time.Hour // 5 days
	MaxLogFiles    = 10 // Keep max 10 rotated files
)

// LogRotator handles log file rotation and cleanup
type LogRotator struct {
	logFilePath string
	maxSize     int64
	maxAge      time.Duration
	maxFiles    int
}

// NewLogRotator creates a new log rotator
func NewLogRotator(logFilePath string) *LogRotator {
	return &LogRotator{
		logFilePath: logFilePath,
		maxSize:     MaxLogFileSize,
		maxAge:      MaxLogAge,
		maxFiles:    MaxLogFiles,
	}
}

// ShouldRotate checks if log file needs rotation
func (lr *LogRotator) ShouldRotate() (bool, error) {
	info, err := os.Stat(lr.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // File doesn't exist, no rotation needed
		}
		return false, err
	}

	// Check file size
	if info.Size() >= lr.maxSize {
		return true, nil
	}

	// Check file age
	if time.Since(info.ModTime()) >= lr.maxAge {
		return true, nil
	}

	return false, nil
}

// RotateLog rotates the current log file
func (lr *LogRotator) RotateLog() error {
	// Create logs directory if it doesn't exist
	logDir := filepath.Dir(lr.logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Check if current log file exists
	if _, err := os.Stat(lr.logFilePath); os.IsNotExist(err) {
		// Create new empty log file
		return lr.createEmptyLogFile()
	}

	// Generate rotated filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	rotatedName := fmt.Sprintf("%s.%s", lr.logFilePath, timestamp)

	// Rename current log file
	if err := os.Rename(lr.logFilePath, rotatedName); err != nil {
		return fmt.Errorf("failed to rotate log file: %v", err)
	}

	// Create new empty log file
	if err := lr.createEmptyLogFile(); err != nil {
		return fmt.Errorf("failed to create new log file after rotation: %v", err)
	}

	// Clean up old log files
	if err := lr.cleanupOldLogs(); err != nil {
		// Log cleanup failure but don't fail the rotation
		fmt.Printf("Warning: failed to cleanup old logs: %v\n", err)
	}

	return nil
}

// createEmptyLogFile creates a new empty log file
func (lr *LogRotator) createEmptyLogFile() error {
	file, err := os.Create(lr.logFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write initial log entry
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	initialEntry := fmt.Sprintf("INFO: %s Log file created/rotated\n", timestamp)
	_, err = file.WriteString(initialEntry)
	return err
}

// cleanupOldLogs removes old log files based on age and count limits
func (lr *LogRotator) cleanupOldLogs() error {
	logDir := filepath.Dir(lr.logFilePath)
	logBaseName := filepath.Base(lr.logFilePath)

	// Find all rotated log files
	pattern := fmt.Sprintf("%s.*", logBaseName)
	matches, err := filepath.Glob(filepath.Join(logDir, pattern))
	if err != nil {
		return err
	}

	var rotatedFiles []LogFileInfo
	cutoffTime := time.Now().Add(-lr.maxAge)

	for _, match := range matches {
		// Skip the current log file
		if match == lr.logFilePath {
			continue
		}

		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		rotatedFiles = append(rotatedFiles, LogFileInfo{
			Path:    match,
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(rotatedFiles, func(i, j int) bool {
		return rotatedFiles[i].ModTime.After(rotatedFiles[j].ModTime)
	})

	// Remove files that are too old or exceed max count
	for i, file := range rotatedFiles {
		shouldDelete := false

		// Delete if older than max age
		if file.ModTime.Before(cutoffTime) {
			shouldDelete = true
		}

		// Delete if exceeds max file count (keep newest files)
		if i >= lr.maxFiles {
			shouldDelete = true
		}

		if shouldDelete {
			if err := os.Remove(file.Path); err != nil {
				fmt.Printf("Warning: failed to remove old log file %s: %v\n", file.Path, err)
			} else {
				fmt.Printf("Cleaned up old log file: %s\n", file.Path)
			}
		}
	}

	return nil
}

// LogFileInfo holds information about a log file
type LogFileInfo struct {
	Path    string
	ModTime time.Time
	Size    int64
}

// StartRotationScheduler starts a goroutine that checks for log rotation periodically
func (lr *LogRotator) StartRotationScheduler(interval time.Duration, onRotate func()) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if shouldRotate, err := lr.ShouldRotate(); err != nil {
					fmt.Printf("Error checking log rotation: %v\n", err)
				} else if shouldRotate {
					if err := lr.RotateLog(); err != nil {
						fmt.Printf("Error rotating log: %v\n", err)
					} else {
						fmt.Printf("Log rotated successfully at %s\n", time.Now().Format("2006-01-02 15:04:05"))
						if onRotate != nil {
							onRotate()
						}
					}
				}
			}
		}
	}()
}

// GetLogStats returns statistics about log files
func (lr *LogRotator) GetLogStats() (LogStats, error) {
	stats := LogStats{}

	// Current log file stats
	if info, err := os.Stat(lr.logFilePath); err == nil {
		stats.CurrentSize = info.Size()
		stats.CurrentAge = time.Since(info.ModTime())
	}

	// Count rotated files
	logDir := filepath.Dir(lr.logFilePath)
	logBaseName := filepath.Base(lr.logFilePath)
	pattern := fmt.Sprintf("%s.*", logBaseName)
	matches, err := filepath.Glob(filepath.Join(logDir, pattern))
	if err != nil {
		return stats, err
	}

	for _, match := range matches {
		if match != lr.logFilePath {
			stats.RotatedCount++
			if info, err := os.Stat(match); err == nil {
				stats.TotalSize += info.Size()
			}
		}
	}

	stats.TotalSize += stats.CurrentSize
	return stats, nil
}

// LogStats holds statistics about log files
type LogStats struct {
	CurrentSize   int64
	CurrentAge    time.Duration
	RotatedCount  int
	TotalSize     int64
}

// FormatSize formats bytes to human readable format
func (s LogStats) FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// String returns a formatted string of log statistics
func (s LogStats) String() string {
	return fmt.Sprintf("Current: %s (age: %v), Rotated files: %d, Total size: %s",
		s.FormatSize(s.CurrentSize),
		s.CurrentAge.Round(time.Minute),
		s.RotatedCount,
		s.FormatSize(s.TotalSize))
}