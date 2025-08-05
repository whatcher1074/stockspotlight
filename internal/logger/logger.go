// File: internal/logger/logger.go
package logger

import (
	"log"
	"os"
)

// Logger provides structured logging.
type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
}

// New creates a new Logger instance.
func New(logPath string) (*Logger, error) {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	infoLogger := log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger := log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return &Logger{
		infoLogger:  infoLogger,
		errorLogger: errorLogger,
	}, nil
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.infoLogger.Println(msg)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	l.errorLogger.Println(msg)
}

// Infof logs a formatted informational message.
func (l *Logger) Infof(format string, v ...interface{}) {
	l.infoLogger.Printf(format, v...)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, v ...interface{}) {
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
