package logger

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "q/types"
)

// Model pricing as of December 2024 (per 1M tokens)
var modelPricing = map[string]ModelPricing{
	"gpt-4.1":       {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4.1-mini":  {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"gpt-4o":        {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4o-mini":   {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"gpt-4-turbo":   {InputPerMillion: 10.00, OutputPerMillion: 30.00},
	"gpt-4":         {InputPerMillion: 30.00, OutputPerMillion: 60.00},
	"gpt-3.5-turbo": {InputPerMillion: 0.50, OutputPerMillion: 1.50},
}

type RequestLogger struct {
	db      *sql.DB
	enabled bool
}

// NewRequestLogger creates a new SQLite-based logger
func NewRequestLogger() (*RequestLogger, error) {
	if os.Getenv("SHELL_AI_DISABLE_LOGGING") != "" {
		return &RequestLogger{enabled: false}, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".shell-ai")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	dbPath := filepath.Join(logDir, "logs.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	logger := &RequestLogger{db: db, enabled: true}
	if err := logger.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return logger, nil
}

// initSchema creates the database schema if it doesn't exist
func (l *RequestLogger) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		name TEXT,
		model TEXT
	);

	CREATE TABLE IF NOT EXISTS responses (
		id TEXT PRIMARY KEY,
		model TEXT,
		prompt TEXT,
		system TEXT,
		response TEXT,
		conversation_id TEXT REFERENCES conversations(id),
		duration_ms INTEGER,
		datetime_utc TEXT,
		input_tokens INTEGER,
		output_tokens INTEGER,
		estimated_cost REAL
	);

	CREATE INDEX IF NOT EXISTS idx_responses_datetime ON responses(datetime_utc);
	CREATE INDEX IF NOT EXISTS idx_responses_conversation ON responses(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_responses_model ON responses(model);
	`

	_, err := l.db.Exec(schema)
	return err
}

// LogResponse logs a single request/response to the database
func (l *RequestLogger) LogResponse(entry LogEntry) error {
	if !l.enabled || l.db == nil {
		return nil
	}

	// Extract system message from messages
	var systemMsg string
	var promptMsg string
	for _, msg := range entry.Messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
		} else if msg.Role == "user" {
			promptMsg = msg.Content
		}
	}

	query := `
		INSERT INTO responses (
			id, model, prompt, system, response,
			conversation_id, duration_ms, datetime_utc,
			input_tokens, output_tokens, estimated_cost
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := l.db.Exec(
		query,
		entry.RequestID,
		entry.Model,
		promptMsg,
		systemMsg,
		entry.Response,
		nil, // conversation_id - can be added later
		entry.DurationMs,
		entry.Timestamp.Format(time.RFC3339),
		entry.PromptTokens,
		entry.CompletionTokens,
		entry.EstimatedCost,
	)

	return err
}

// GetRecentResponses retrieves the N most recent responses
func (l *RequestLogger) GetRecentResponses(limit int) ([]LogEntry, error) {
	if !l.enabled || l.db == nil {
		return nil, nil
	}

	query := `
		SELECT id, model, prompt, system, response,
		       datetime_utc, input_tokens, output_tokens,
		       estimated_cost, duration_ms
		FROM responses
		ORDER BY datetime_utc DESC
		LIMIT ?
	`

	rows, err := l.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var entry LogEntry
		var datetimeStr string
		var systemMsg, promptMsg string

		err := rows.Scan(
			&entry.RequestID,
			&entry.Model,
			&promptMsg,
			&systemMsg,
			&entry.Response,
			&datetimeStr,
			&entry.PromptTokens,
			&entry.CompletionTokens,
			&entry.EstimatedCost,
			&entry.DurationMs,
		)
		if err != nil {
			continue
		}

		// Reconstruct messages
		if systemMsg != "" {
			entry.Messages = append(entry.Messages, Message{Role: "system", Content: systemMsg})
		}
		if promptMsg != "" {
			entry.Messages = append(entry.Messages, Message{Role: "user", Content: promptMsg})
		}

		// Parse timestamp
		entry.Timestamp, _ = time.Parse(time.RFC3339, datetimeStr)

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetDBPath returns the path to the logs database
func (l *RequestLogger) GetDBPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".shell-ai", "logs.db")
}

// Close closes the database connection
func (l *RequestLogger) Close() error {
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

// CalculateCost estimates the cost in USD based on token usage
func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	pricing, ok := modelPricing[model]
	if !ok {
		return 0.0
	}

	inputCost := (float64(promptTokens) / 1_000_000) * pricing.InputPerMillion
	outputCost := (float64(completionTokens) / 1_000_000) * pricing.OutputPerMillion

	return inputCost + outputCost
}

// CreateLogEntry creates a LogEntry with all fields populated
func CreateLogEntry(model string, messages []Message, response string, usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}, requestID string, durationMs int64, err error) LogEntry {
	entry := LogEntry{
		Timestamp:        time.Now().UTC(),
		Model:            model,
		Messages:         messages,
		Response:         response,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		EstimatedCost:    CalculateCost(model, usage.PromptTokens, usage.CompletionTokens),
		RequestID:        requestID,
		DurationMs:       durationMs,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return entry
}
