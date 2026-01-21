package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"WARN", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"off", LevelOff},
		{"OFF", LevelOff},
		{"none", LevelOff},
		{"unknown", LevelInfo}, // Default
		{"", LevelInfo},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("Level.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer

	l := &Logger{
		level:   LevelDebug,
		output:  &buf,
		enabled: true,
	}

	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected 'test message' in output, got: %s", output)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	l := &Logger{
		level:   LevelWarn,
		output:  &buf,
		enabled: true,
	}

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")

	output := buf.String()

	// Debug and Info should be filtered out
	if strings.Contains(output, "debug message") {
		t.Error("debug message should be filtered")
	}
	if strings.Contains(output, "info message") {
		t.Error("info message should be filtered")
	}

	// Warn and Error should appear
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should appear")
	}
	if !strings.Contains(output, "error message") {
		t.Error("error message should appear")
	}
}

func TestLoggerDisabled(t *testing.T) {
	var buf bytes.Buffer

	l := &Logger{
		level:   LevelDebug,
		output:  &buf,
		enabled: false,
	}

	l.Info("test message")

	if buf.Len() > 0 {
		t.Error("disabled logger should not write output")
	}
}

func TestLoggerPrefix(t *testing.T) {
	var buf bytes.Buffer

	l := &Logger{
		level:   LevelDebug,
		output:  &buf,
		enabled: true,
		prefix:  "SSH",
	}

	l.Info("connecting")

	output := buf.String()
	if !strings.Contains(output, "[SSH]") {
		t.Errorf("expected [SSH] prefix in output, got: %s", output)
	}
}

func TestLoggerSetLevel(t *testing.T) {
	var buf bytes.Buffer

	l := &Logger{
		level:   LevelError,
		output:  &buf,
		enabled: true,
	}

	l.Info("should not appear")

	l.SetLevel(LevelInfo)
	l.Info("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("first message should be filtered")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("second message should appear")
	}
}

func TestLoggerFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := New(Config{
		Level:    "debug",
		FilePath: logPath,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	l.Info("test log message")
	l.Close()

	// Read log file
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "test log message") {
		t.Errorf("expected message in log file, got: %s", content)
	}
}

func TestLoggerFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "secure.log")

	l, err := New(Config{
		Level:    "info",
		FilePath: logPath,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	l.Info("test")
	l.Close()

	// Check file permissions
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}
}

func TestDefaultLogPath(t *testing.T) {
	path := DefaultLogPath()

	if !strings.Contains(path, "thop") {
		t.Errorf("expected 'thop' in path, got: %s", path)
	}
	if !strings.HasSuffix(path, "thop.log") {
		t.Errorf("expected path to end with 'thop.log', got: %s", path)
	}
}

func TestLoggerFormatting(t *testing.T) {
	var buf bytes.Buffer

	l := &Logger{
		level:   LevelDebug,
		output:  &buf,
		enabled: true,
	}

	l.Info("value: %d, name: %s", 42, "test")

	output := buf.String()
	if !strings.Contains(output, "value: 42") {
		t.Errorf("expected formatted value in output, got: %s", output)
	}
	if !strings.Contains(output, "name: test") {
		t.Errorf("expected formatted name in output, got: %s", output)
	}
}

func TestDefaultLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "default.log")

	err := Init(Config{
		Level:    "debug",
		FilePath: logPath,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	Info("package level log")

	// Close and read
	Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), "package level log") {
		t.Error("expected message in log file")
	}
}
