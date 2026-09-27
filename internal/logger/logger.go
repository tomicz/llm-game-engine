// Package logger keeps the in-game terminal log and the engine log files.
package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// LogFilePath is the terminal/chat log (every terminal line). Appended to, never cleared.
	LogFilePath = "logs/terminal.txt"
	// EngineLogFilePath is the engine log (raylib trace output, engine errors, and stderr,
	// including crash dumps). Appended to, never cleared.
	EngineLogFilePath = "logs/engine_log.txt"
	// maxLines caps in-memory terminal lines.
	maxLines = 1000
	stamp    = "2006-01-02 15:04:05"
)

// Logger stores terminal lines in memory and in terminal.txt, and writes engine output to
// engine_log.txt. It is safe for concurrent use.
type Logger struct {
	mu        sync.Mutex
	lines     []string
	termLog   *os.File
	engineLog *os.File
}

// New opens the log files under logs/ (created if needed) and tees stderr into the engine log so
// runtime crash dumps are captured too. Logging still works in memory if the files can't be opened.
func New() *Logger {
	_ = os.MkdirAll(filepath.Dir(LogFilePath), 0o755)
	l := &Logger{}
	l.termLog, _ = os.OpenFile(LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	l.engineLog, _ = os.OpenFile(EngineLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if l.engineLog != nil {
		teeStderr(l.engineLog)
	}
	return l
}

// teeStderr redirects os.Stderr through a pipe whose contents go to both the original stderr and w.
func teeStderr(w io.Writer) {
	original := os.Stderr
	r, pw, err := os.Pipe()
	if err != nil {
		return
	}
	os.Stderr = pw
	go func() {
		_, _ = io.Copy(io.MultiWriter(original, w), r)
		r.Close()
	}()
}

// raylibLevels names raylib trace log levels 0–6.
var raylibLevels = [...]string{"ALL", "TRACE", "DEBUG", "INFO", "WARNING", "ERROR", "FATAL"}

func levelName(level int) string {
	if level >= 0 && level < len(raylibLevels) {
		return raylibLevels[level]
	}
	return "LOG"
}

// Log adds a timestamped line to the terminal (capped in memory) and to terminal.txt.
func (l *Logger) Log(line string) {
	stamped := "[" + time.Now().Format(stamp) + "] " + line
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, stamped)
	if len(l.lines) > maxLines {
		l.lines = l.lines[len(l.lines)-maxLines:]
	}
	if l.termLog != nil {
		_, _ = l.termLog.WriteString(stamped + "\n")
	}
}

// LogEngine appends a line to engine_log.txt. It matches raylib's trace log callback signature.
func (l *Logger) LogEngine(level int, msg string) {
	line := "[" + time.Now().Format(stamp) + "] [" + levelName(level) + "] " + msg + "\n"
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.engineLog != nil {
		_, _ = l.engineLog.WriteString(line)
	}
}

// Error appends an engine error to engine_log.txt.
func (l *Logger) Error(msg string) {
	l.LogEngine(5, msg) // 5 = raylib LOG_ERROR
}

// Lines returns a copy of the terminal lines.
func (l *Logger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

// Tail returns a copy of the last n terminal lines.
func (l *Logger) Tail(n int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines[max(len(l.lines)-n, 0):]...)
}
