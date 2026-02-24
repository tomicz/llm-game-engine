package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// LogFilePath is the terminal/chat log file (user input only). Not cleared on start.
	LogFilePath = "logs/terminal.txt"
	// EngineLogFilePath is the engine log file (raylib INFO/WARNING/ERROR and engine errors). Persists after exit.
	EngineLogFilePath = "logs/engine_log.txt"
)

// Logger stores terminal lines in memory and writes terminal logs to terminal.txt.
// Engine/raylib output is appended to engine_log.txt and persists across game runs.
type Logger struct {
	mu    sync.Mutex
	lines []string
}

// New returns a new Logger and ensures the logs directory exists. Engine log is not cleared; output persists.
// Tees stderr to the engine log file so runtime crash dumps (e.g. SIGSEGV) are also written there.
func New() *Logger {
	dir := filepath.Dir(LogFilePath)
	_ = os.MkdirAll(dir, 0755)
	teeStderrToEngineLog(dir)
	return &Logger{lines: make([]string, 0)}
}

// teeStderrToEngineLog redirects stderr through a pipe; a goroutine copies to both original stderr and engine_log.txt.
func teeStderrToEngineLog(logsDir string) {
	engineLogPath := filepath.Join(logsDir, "engine_log.txt")
	f, err := os.OpenFile(engineLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		f.Close()
		return
	}
	os.Stderr = w
	go func() {
		_, _ = io.Copy(io.MultiWriter(originalStderr, f), r)
		r.Close()
		f.Close()
	}()
}

// logLevelName maps raylib trace log level (0–6) to a string label.
func logLevelName(level int) string {
	switch level {
	case 0:
		return "ALL"
	case 1:
		return "TRACE"
	case 2:
		return "DEBUG"
	case 3:
		return "INFO"
	case 4:
		return "WARNING"
	case 5:
		return "ERROR"
	case 6:
		return "FATAL"
	default:
		return "LOG"
	}
}

// Log appends a terminal/chat line to memory and to logs/terminal.txt only. Use for user input from the terminal.
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

// LogEngine appends a line to logs/engine_log.txt. Used by the raylib trace callback (INFO, WARNING, etc.). Persists after exit.
func (l *Logger) LogEngine(logType int, msg string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	level := logLevelName(logType)
	line := "[" + ts + "] [" + level + "] " + msg + "\n"

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(EngineLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}

// Error appends an engine error to logs/engine_log.txt. Persists after the game exits; use for engine errors only.
func (l *Logger) Error(msg string) {
	l.LogEngine(5, msg) // 5 = ERROR in raylib
}

// Lines returns a copy of all stored terminal lines (from Log, not game logs).
func (l *Logger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.lines))
	copy(out, l.lines)
	return out
}
