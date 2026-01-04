package types

import "time"

type ModelConfig struct {
	ModelName string    `yaml:"name"`
	Endpoint  string    `yaml:"endpoint"`
	Auth      string    `yaml:"auth_env_var"`
	OrgID     string    `yaml:"org_env_var,omitempty"`
	Prompt    []Message `yaml:"prompt"`
}

type Message struct {
	Role    string `yaml:"role" json:"role"`
	Content string `yaml:"content" json:"content"`
}

type Preferences struct {
	DefaultModel string `yaml:"default_model"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type Payload struct {
	Model         string         `json:"model"`
	Prompt        string         `json:"prompt,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Temperature   float32        `json:"temperature,omitempty"`
	Messages      []Message      `json:"messages"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

type ResponseData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type LogEntry struct {
	Timestamp        time.Time `json:"timestamp"`
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Response         string    `json:"response"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	EstimatedCost    float64   `json:"estimated_cost_usd"`
	RequestID        string    `json:"request_id,omitempty"`
	DurationMs       int64     `json:"duration_ms,omitempty"`
	Error            string    `json:"error,omitempty"`
}

type ModelPricing struct {
	InputPerMillion  float64
	OutputPerMillion float64
}
