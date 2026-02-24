package logger

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogFilePath is the path to the terminal log file, relative to the working directory (project root when run via go run ./cmd/game).
const LogFilePath = "logs/terminal.txt"

// Logger stores lines of text (e.g. from terminal input) in memory and appends them to a file on disk.
type Logger struct {
	mu    sync.Mutex
	lines []string
}

// New returns a new Logger and ensures the logs directory exists.
func New() *Logger {
	dir := filepath.Dir(LogFilePath)
	_ = os.MkdirAll(dir, 0755)
	return &Logger{lines: make([]string, 0)}
}

// Log appends a line to the logger and appends it to the log file on disk. Each entry is prefixed with [timestamp] using computer time.
func (l *Logger) Log(line string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	stamped := "[" + ts + "] " + line

	l.mu.Lock()
	l.lines = append(l.lines, stamped)
	l.mu.Unlock()

	f, err := os.OpenFile(LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(stamped + "\n")
	_ = f.Close()
}

// Lines returns a copy of all stored lines.
func (l *Logger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.lines))
	copy(out, l.lines)
	return out
}
