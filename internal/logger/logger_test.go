package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestLogAndTail(t *testing.T) {
	t.Chdir(t.TempDir())
	l := &Logger{} // no files, no stderr tee
	for i := range maxLines + 5 {
		l.Log(fmt.Sprint(i))
	}
	lines := l.Lines()
	if len(lines) != maxLines {
		t.Fatalf("kept %d lines, want %d", len(lines), maxLines)
	}
	if !strings.HasSuffix(lines[0], "] 5") {
		t.Errorf("oldest line = %q, want the 6th logged", lines[0])
	}
	tail := l.Tail(2)
	if len(tail) != 2 || !strings.HasSuffix(tail[1], fmt.Sprint(maxLines+4)) {
		t.Errorf("Tail(2) = %q", tail)
	}
	if got := (&Logger{}).Tail(3); len(got) != 0 {
		t.Errorf("Tail on empty = %q", got)
	}
}

func TestFilesAppend(t *testing.T) {
	t.Chdir(t.TempDir())
	stderr := os.Stderr
	t.Cleanup(func() { os.Stderr = stderr })

	l := New()
	l.Log("hello")
	l.LogEngine(4, "careful")
	l.Error("broken")
	l.LogEngine(42, "odd")

	term, _ := os.ReadFile(LogFilePath)
	if !strings.HasSuffix(string(term), "] hello\n") {
		t.Errorf("terminal.txt = %q", term)
	}
	engine, _ := os.ReadFile(EngineLogFilePath)
	for _, want := range []string{"[WARNING] careful", "[ERROR] broken", "[LOG] odd"} {
		if !strings.Contains(string(engine), want) {
			t.Errorf("engine_log.txt missing %q:\n%s", want, engine)
		}
	}
}

func TestConcurrentLog(t *testing.T) {
	l := &Logger{}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 50 {
				l.Log(fmt.Sprint(i, j))
			}
		})
	}
	wg.Wait()
	if n := len(l.Lines()); n != 400 {
		t.Errorf("got %d lines, want 400", n)
	}
}
