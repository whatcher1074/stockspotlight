// File: internal/logger/logger.go
package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Logger provides structured logging with rotation.
type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	file        *os.File
	logPath     string
	rotator     *LogRotator
}

// New creates a new Logger instance with log rotation.
func New(logPath string) (*Logger, error) {
	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// Open log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	// Create multi-writer for both file and console (optional)
	// Comment out the next line if you only want file logging
	multiWriter := io.MultiWriter(file, os.Stdout)

	// Create logger instance
	logger := &Logger{
		infoLogger:  log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		file:        file,
		logPath:     logPath,
		rotator:     NewLogRotator(logPath),
	}

	// Start rotation scheduler (check every 10 minutes)
	logger.rotator.StartRotationScheduler(10*time.Minute, func() {
		logger.reopenLogFile()
	})

	// Log initial startup message
	logger.Info("Logger initialized with rotation enabled")
	logger.logRotationInfo()

	return logger, nil
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.checkAndRotate()
	l.infoLogger.Println(msg)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	l.checkAndRotate()
	l.errorLogger.Println(msg)
}

// Infof logs a formatted informational message.
func (l *Logger) Infof(format string, v ...interface{}) {
	l.checkAndRotate()
	l.infoLogger.Printf(format, v...)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.checkAndRotate()
	l.errorLogger.Printf(format, v...)
}

// Fatal logs a fatal error and exits.
func (l *Logger) Fatal(v ...interface{}) {
	l.errorLogger.Fatal(v...)
}

// Fatalf logs a formatted fatal error and exits.
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.errorLogger.Fatalf(format, v...)
}

// checkAndRotate checks if rotation is needed before each log write
func (l *Logger) checkAndRotate() {
	if shouldRotate, err := l.rotator.ShouldRotate(); err == nil && shouldRotate {
		l.rotateNow()
	}
}

// rotateNow performs immediate log rotation
func (l *Logger) rotateNow() {
	// Close current file
	if l.file != nil {
		l.file.Close()
	}

	// Perform rotation
	if err := l.rotator.RotateLog(); err != nil {
		// Fallback: try to reopen current file
		l.reopenLogFile()
		return
	}

	// Reopen the new file
	l.reopenLogFile()
	l.Info("Log rotation completed")
	l.logRotationInfo()
}

// reopenLogFile reopens the log file after rotation
func (l *Logger) reopenLogFile() {
	file, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stdout only
		l.infoLogger.SetOutput(os.Stdout)
		l.errorLogger.SetOutput(os.Stdout)
		l.Error("Failed to reopen log file, falling back to stdout")
		return
	}

	l.file = file

	// Update loggers - comment out multiWriter line if you only want file logging
	multiWriter := io.MultiWriter(file, os.Stdout)
	l.infoLogger.SetOutput(multiWriter)
	l.errorLogger.SetOutput(multiWriter)
}

// logRotationInfo logs information about rotation settings
func (l *Logger) logRotationInfo() {
	if stats, err := l.rotator.GetLogStats(); err == nil {
		l.Infof("Log stats: %s", stats.String())
	}
	l.Infof("Log rotation: max size %s, max age %v, max files %d",
		LogStats{}.FormatSize(MaxLogFileSize), MaxLogAge, MaxLogFiles)
}

// GetStats returns current log statistics (new method)
func (l *Logger) GetStats() (LogStats, error) {
	return l.rotator.GetLogStats()
}

// ForceRotate forces immediate log rotation (new method)
func (l *Logger) ForceRotate() error {
	l.Info("Manual log rotation requested")
	l.rotateNow()
	return nil
}

// CleanupOldLogs manually triggers cleanup (new method)
func (l *Logger) CleanupOldLogs() error {
	l.Info("Manual log cleanup requested")
	return l.rotator.cleanupOldLogs()
}

// Close closes the logger (new method)
func (l *Logger) Close() error {
	l.Info("Logger shutting down")
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}