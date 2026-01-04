package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	. "q/types"
	"strings"
	"time"

	"q/logger"
)

type LLMClient struct {
	config   ModelConfig
	messages []Message

	StreamCallback func(string, error)

	httpClient *http.Client
	logger     *logger.RequestLogger
}

func NewLLMClient(config ModelConfig) *LLMClient {
	// Initialize logger (best effort, non-fatal if it fails)
	reqLogger, _ := logger.NewRequestLogger()

	return &LLMClient{
		config:   config,
		messages: append([]Message(nil), config.Prompt...),

		httpClient: &http.Client{
			Timeout: time.Second * 120,
		},
		logger: reqLogger,
	}
}

func (c *LLMClient) createRequest(payload Payload) (*http.Request, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	req, err := http.NewRequest("POST", c.config.Endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if strings.Contains(c.config.Endpoint, "openai.azure.com") {
		req.Header.Set("Api-Key", c.config.Auth)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.config.Auth)
	}
	if c.config.OrgID != "" {
		req.Header.Set("OpenAI-Organization", c.config.OrgID)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c *LLMClient) Query(query string) (string, error) {
	startTime := time.Now()
	messages := c.messages
	messages = append(messages, Message{Role: "user", Content: query})

	payload := Payload{
		Model:       c.config.ModelName,
		Messages:    messages,
		Temperature: 0,
		Stream:      true,
		StreamOptions: &StreamOptions{IncludeUsage: true},
	}

	message, usage, requestID, err := c.callStream(payload)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		// Log error case
		if c.logger != nil {
			logEntry := logger.CreateLogEntry(
				c.config.ModelName,
				messages,
				"",
				usage,
				requestID,
				durationMs,
				err,
			)
			if logErr := c.logger.LogResponse(logEntry); logErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write log: %v\n", logErr)
			}
		}
		return "", err
	}

	c.messages = append(c.messages, message)

	// Log successful case
	if c.logger != nil {
		logEntry := logger.CreateLogEntry(
			c.config.ModelName,
			messages,
			message.Content,
			usage,
			requestID,
			durationMs,
			nil,
		)
		if logErr := c.logger.LogResponse(logEntry); logErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write log: %v\n", logErr)
		}
	}

	return message.Content, nil
}

func (c *LLMClient) processStream(resp *http.Response) (string, struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}, string, error) {
	counter := 0
	streamReader := bufio.NewReader(resp.Body)
	totalData := ""
	var usage struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}
	var requestID string

	for {
		line, err := streamReader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "data: [DONE]" {
			break
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimPrefix(line, "data:")

			var responseData ResponseData
			err = json.Unmarshal([]byte(payload), &responseData)
			if err != nil {
				fmt.Println("Error parsing data:", err)
				continue
			}

			// Capture request ID from first chunk
			if requestID == "" && responseData.ID != "" {
				requestID = responseData.ID
			}

			// Capture usage data from final chunk
			if responseData.Usage.TotalTokens > 0 {
				usage.PromptTokens = responseData.Usage.PromptTokens
				usage.CompletionTokens = responseData.Usage.CompletionTokens
				usage.TotalTokens = responseData.Usage.TotalTokens
			}

			if len(responseData.Choices) == 0 {
				continue
			}
			content := responseData.Choices[0].Delta.Content
			if counter < 2 && strings.Count(content, "\n") > 0 {
				continue
			}
			totalData += content
			c.StreamCallback(totalData, nil)
			counter++
		}
	}
	return totalData, usage, requestID, nil
}

func (c *LLMClient) callStream(payload Payload) (Message, struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}, string, error) {
	var emptyUsage struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}

	req, err := c.createRequest(payload)
	if err != nil {
		return Message{}, emptyUsage, "", fmt.Errorf("failed to create the request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Message{}, emptyUsage, "", fmt.Errorf("failed to make the API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return Message{}, emptyUsage, "", fmt.Errorf("API request failed: %s", resp.Status)
	}
	content, usage, requestID, err := c.processStream(resp)
	return Message{Role: "assistant", Content: content}, usage, requestID, err
}
