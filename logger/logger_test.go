package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "q/types"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model      string
		prompt     int
		completion int
		expected   float64
	}{
		{"gpt-4.1", 1000, 500, 0.0025 + 0.0050},              // 2.50/M * 0.001M + 10.00/M * 0.0005M = 0.0075
		{"gpt-4.1-mini", 10000, 5000, 0.0015 + 0.0030},       // 0.15/M * 0.01M + 0.60/M * 0.005M = 0.0045
		{"gpt-4o", 2000, 1000, 0.0050 + 0.0100},              // 2.50/M * 0.002M + 10.00/M * 0.001M = 0.015
		{"unknown-model", 1000, 500, 0.0},                    // Unknown model returns 0
		{"gpt-3.5-turbo", 100000, 50000, 0.05 + 0.075},       // 0.50/M * 0.1M + 1.50/M * 0.05M = 0.125
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := CalculateCost(tt.model, tt.prompt, tt.completion)
			if result != tt.expected {
				t.Errorf("CalculateCost(%s, %d, %d) = %f; want %f",
					tt.model, tt.prompt, tt.completion, result, tt.expected)
			}
		})
	}
}

func TestLogEntry(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.jsonl")

	logger := &RequestLogger{logFilePath: logPath}

	entry := LogEntry{
		Timestamp:        time.Now().UTC(),
		Model:            "gpt-4.1-mini",
		Messages:         []Message{{Role: "user", Content: "test query"}},
		Response:         "test response",
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		EstimatedCost:    0.000009,
		RequestID:        "test-req-123",
	}

	if err := logger.Log(entry); err != nil {
		t.Fatalf("Failed to log entry: %v", err)
	}

	// Verify file exists and contains data
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Log file is empty")
	}

	// Verify it's valid JSON
	var loggedEntry LogEntry
	if err := json.Unmarshal(data[:len(data)-1], &loggedEntry); err != nil { // Remove trailing newline
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	// Verify key fields
	if loggedEntry.Model != entry.Model {
		t.Errorf("Model mismatch: got %s, want %s", loggedEntry.Model, entry.Model)
	}
	if loggedEntry.TotalTokens != entry.TotalTokens {
		t.Errorf("TotalTokens mismatch: got %d, want %d", loggedEntry.TotalTokens, entry.TotalTokens)
	}
}

func TestCreateLogEntry(t *testing.T) {
	usage := struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello"},
	}

	entry := CreateLogEntry(
		"gpt-4.1-mini",
		messages,
		"Hi there!",
		usage,
		"req-123",
		nil,
	)

	// Verify fields are populated correctly
	if entry.Model != "gpt-4.1-mini" {
		t.Errorf("Model mismatch: got %s, want gpt-4.1-mini", entry.Model)
	}
	if entry.PromptTokens != 100 {
		t.Errorf("PromptTokens mismatch: got %d, want 100", entry.PromptTokens)
	}
	if entry.CompletionTokens != 50 {
		t.Errorf("CompletionTokens mismatch: got %d, want 50", entry.CompletionTokens)
	}
	if entry.TotalTokens != 150 {
		t.Errorf("TotalTokens mismatch: got %d, want 150", entry.TotalTokens)
	}
	if entry.RequestID != "req-123" {
		t.Errorf("RequestID mismatch: got %s, want req-123", entry.RequestID)
	}
	if entry.Error != "" {
		t.Errorf("Error should be empty, got %s", entry.Error)
	}

	// Verify cost calculation
	expectedCost := CalculateCost("gpt-4.1-mini", 100, 50)
	if entry.EstimatedCost != expectedCost {
		t.Errorf("EstimatedCost mismatch: got %f, want %f", entry.EstimatedCost, expectedCost)
	}
}

func TestNewRequestLogger(t *testing.T) {
	// Set env var to disable logging
	os.Setenv("SHELL_AI_DISABLE_LOGGING", "1")
	defer os.Unsetenv("SHELL_AI_DISABLE_LOGGING")

	logger, err := NewRequestLogger()
	if err != nil {
		t.Fatalf("NewRequestLogger should not error when disabled: %v", err)
	}
	if logger != nil {
		t.Error("Logger should be nil when SHELL_AI_DISABLE_LOGGING is set")
	}
}
