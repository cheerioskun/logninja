package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Logger provides a centralized logging mechanism for LogNinja
type Logger struct {
	warningLogger *log.Logger
	debugLogger   *log.Logger
	errorLogger   *log.Logger
	file          *os.File
	mu            sync.Mutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// GetLogger returns the default logger instance (singleton pattern)
func GetLogger() *Logger {
	once.Do(func() {
		var err error
		defaultLogger, err = NewLogger("/tmp/logninja.out")
		if err != nil {
			// Fallback to stderr if we can't create the log file
			log.Printf("Failed to create log file, falling back to stderr: %v", err)
			defaultLogger = &Logger{
				warningLogger: log.New(os.Stderr, "[WARN] ", log.LstdFlags|log.Lshortfile),
				debugLogger:   log.New(os.Stderr, "[DEBUG] ", log.LstdFlags|log.Lshortfile),
				errorLogger:   log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile),
				file:          nil,
			}
		}
	})
	return defaultLogger
}

// NewLogger creates a new logger that writes to the specified file
func NewLogger(logPath string) (*Logger, error) {
	// Ensure the directory exists
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open or create the log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Logger{
		warningLogger: log.New(file, "[WARN] ", log.LstdFlags|log.Lshortfile),
		debugLogger:   log.New(file, "[DEBUG] ", log.LstdFlags|log.Lshortfile),
		errorLogger:   log.New(file, "[ERROR] ", log.LstdFlags|log.Lshortfile),
		file:          file,
	}, nil
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warningLogger.Printf(format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugLogger.Printf(format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorLogger.Printf(format, args...)
}

// Close closes the log file (if any)
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Convenience functions for the default logger
func Warning(format string, args ...interface{}) {
	GetLogger().Warning(format, args...)
}

func Debug(format string, args ...interface{}) {
	GetLogger().Debug(format, args...)
}

func Error(format string, args ...interface{}) {
	GetLogger().Error(format, args...)
}
