package logger

import (
	"bytes"
	"strings"
	"testing"
)

func capture(fn func()) string {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetLevel(DEBUG)
	fn()
	SetOutput(output)
	return buf.String()
}

func TestDebug(t *testing.T) {
	out := capture(func() { Debug("test %d", 42) })
	if !strings.Contains(out, "[DEBUG]") {
		t.Errorf("expected [DEBUG] in output, got: %s", out)
	}
	if !strings.Contains(out, "test 42") {
		t.Errorf("expected 'test 42' in output, got: %s", out)
	}
}

func TestInfo(t *testing.T) {
	out := capture(func() { Info("info msg") })
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("expected [INFO] in output, got: %s", out)
	}
}

func TestWarn(t *testing.T) {
	out := capture(func() { Warn("warn msg") })
	if !strings.Contains(out, "[WARN]") {
		t.Errorf("expected [WARN] in output, got: %s", out)
	}
}

func TestError(t *testing.T) {
	out := capture(func() { Error("error msg") })
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("expected [ERROR] in output, got: %s", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetLevel(ERROR)

	Debug("should not appear")
	Info("should not appear")
	Warn("should not appear")
	Error("should appear")

	SetLevel(currentLevel)

	if strings.Contains(buf.String(), "should not appear") {
		t.Error("DEBUG/INFO/WARN should be filtered out at ERROR level")
	}
	if !strings.Contains(buf.String(), "should appear") {
		t.Error("ERROR should appear at ERROR level")
	}
}

func TestErrorf(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetLevel(ERROR)

	err := Errorf("wrapped error: %d", 500)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "wrapped error: 500") {
		t.Errorf("wrong error message: %v", err)
	}
	if !strings.Contains(buf.String(), "[ERROR]") {
		t.Errorf("expected [ERROR] in log output")
	}
}

func TestSetLevelGetLevel(t *testing.T) {
	original := GetLevel()

	SetLevel(DEBUG)
	if GetLevel() != DEBUG {
		t.Errorf("expected DEBUG level")
	}

	SetLevel(WARN)
	if GetLevel() != WARN {
		t.Errorf("expected WARN level")
	}

	SetLevel(original)
}
