package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
)

const openRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

// TokenUsage tracks API token usage
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	Cost         float64 // Actual cost from OpenRouter API
}

// ClaudeAgent wraps the OpenRouter API client
type ClaudeAgent struct {
	apiKey     string
	httpClient *http.Client
	ctx        context.Context
	model      string
}

// NewClaudeAgent creates a new OpenRouter API client
// If model is empty, defaults to "qwen/qwen3-coder:free" - best free coding model on OpenRouter
func NewClaudeAgent(apiKey, model string) *ClaudeAgent {
	if model == "" {
		model = "qwen/qwen3-coder:free" // Best free coding model on OpenRouter
	}

	return &ClaudeAgent{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		ctx:        context.Background(),
		model:      model,
	}
}

// AgentMessage represents a message in the conversation
type AgentMessage struct {
	Role    string
	Content string
}

// OpenRouter API request/response structures
type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterRequest struct {
	Model          string              `json:"model"`
	Messages       []openRouterMessage `json:"messages"`
	MaxTokens      int                 `json:"max_tokens,omitempty"`
	Temperature    float64             `json:"temperature,omitempty"`
	ResponseFormat *responseFormat     `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openRouterUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type openRouterChoice struct {
	Message      openRouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type openRouterResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []openRouterChoice `json:"choices"`
	Usage   openRouterUsage    `json:"usage"`
}

type openRouterError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// SendMessageWithStructuredOutput sends a message with optional JSON schema for structured output
// If useStructuredOutput is true, it attempts JSON schema first, then falls back to regular format
func (ca *ClaudeAgent) SendMessageWithStructuredOutput(messages []AgentMessage, systemPrompt string, useStructuredOutput bool) (string, TokenUsage, error) {
	if useStructuredOutput {
		// Try with structured output first
		response, usage, err := ca.sendMessageInternal(messages, systemPrompt, true)
		if err == nil {
			return response, usage, nil
		}

		// If structured output failed, log and retry without it
		log.Printf("⚠️  Structured output not supported by model, falling back to markdown format")
	}

	// Use regular format (no structured output)
	return ca.sendMessageInternal(messages, systemPrompt, false)
}

// SendMessage sends a message to OpenRouter and gets a response with usage tracking
func (ca *ClaudeAgent) SendMessage(messages []AgentMessage, systemPrompt string) (string, TokenUsage, error) {
	return ca.sendMessageInternal(messages, systemPrompt, false)
}

// sendMessageInternal is the internal implementation that handles both structured and regular output
func (ca *ClaudeAgent) sendMessageInternal(messages []AgentMessage, systemPrompt string, useStructuredOutput bool) (string, TokenUsage, error) {
	// Build messages array with system prompt first
	var apiMessages []openRouterMessage

	// Add system prompt as first message
	if systemPrompt != "" {
		apiMessages = append(apiMessages, openRouterMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Add conversation messages
	for _, msg := range messages {
		apiMessages = append(apiMessages, openRouterMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Create request
	reqBody := openRouterRequest{
		Model:     ca.model,
		Messages:  apiMessages,
		MaxTokens: 8096,
	}

	// Add structured output schema if requested
	if useStructuredOutput {
		reqBody.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchema{
				Name:   "code_changes",
				Strict: true,
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"summary": map[string]any{
							"type":        "string",
							"description": "A brief summary of the changes made",
						},
						"files": map[string]any{
							"type":        "array",
							"description": "List of files to create or modify",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"path": map[string]any{
										"type":        "string",
										"description": "File path relative to repository root",
									},
									"content": map[string]any{
										"type":        "string",
										"description": "Complete file content",
									},
								},
								"required":             []string{"path", "content"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"summary", "files"},
					"additionalProperties": false,
				},
			},
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ca.ctx, "POST", openRouterAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ca.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/yourusername/NyteBubo") // Optional: for OpenRouter analytics
	req.Header.Set("X-Title", "NyteBubo GitHub Agent")                        // Optional: for OpenRouter analytics

	// Send request
	resp, err := ca.httpClient.Do(req)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var errResp openRouterError
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			return "", TokenUsage{}, fmt.Errorf("OpenRouter API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", TokenUsage{}, fmt.Errorf("OpenRouter API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp openRouterResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", TokenUsage{}, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract response text
	if len(apiResp.Choices) == 0 {
		return "", TokenUsage{}, fmt.Errorf("no choices in response")
	}

	responseText := apiResp.Choices[0].Message.Content

	// Get actual cost from OpenRouter response header
	actualCost := 0.0
	costHeader := resp.Header.Get("X-OpenRouter-Generation-Cost")
	if costHeader != "" {
		if parsedCost, err := strconv.ParseFloat(costHeader, 64); err == nil {
			actualCost = parsedCost
		}
	} else {
		log.Printf("⚠️  Warning: OpenRouter did not provide cost data in response header")
	}

	// Track token usage
	usage := TokenUsage{
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
		TotalTokens:  apiResp.Usage.TotalTokens,
		Cost:         actualCost,
	}

	// Get model name from response (useful when using auto-routing)
	modelUsed := apiResp.Model
	if modelUsed == "" {
		modelUsed = ca.model
	}

	// Log usage information
	log.Printf("📊 OpenRouter API [%s] - Input: %d | Output: %d | Total: %d tokens | Cost: $%.4f",
		modelUsed, usage.InputTokens, usage.OutputTokens, usage.TotalTokens, usage.Cost)

	return responseText, usage, nil
}


// AnalyzeIssue asks Claude to analyze a GitHub issue
func (ca *ClaudeAgent) AnalyzeIssue(title, body string) (string, TokenUsage, error) {
	systemPrompt := `You are a helpful AI coding assistant that analyzes GitHub issues.
Your job is to:
1. Understand what the issue is asking for
2. Ask clarifying questions if anything is unclear
3. Provide a clear summary of what needs to be done

Be concise and professional.`

	userMessage := fmt.Sprintf(`Please analyze this GitHub issue:

Title: %s

Description:
%s

Provide:
1. A clear summary of what this issue is asking for
2. Any clarifying questions you have
3. If everything is clear, confirm you understand and are ready to create a PR`, title, body)

	messages := []AgentMessage{
		{Role: "user", Content: userMessage},
	}

	return ca.SendMessage(messages, systemPrompt)
}

// GenerateCode asks Claude to generate code for a specific task
// It attempts to use structured JSON output for compatible models, with markdown fallback
func (ca *ClaudeAgent) GenerateCode(task, context, language string, conversationHistory []AgentMessage) (string, TokenUsage, error) {
	systemPrompt := fmt.Sprintf(`You are an expert software engineer working on a GitHub issue.
You have full access to the repository and need to implement the requested changes.

Programming Language: %s
Repository Context: %s

Your task: %s

IMPORTANT - Response Format:
Provide a summary of your changes followed by the file changes.

For each file you create or modify, use this format:

` + "```" + `%s path/to/file.ext
complete file content here
` + "```" + `

Examples:

` + "```" + `markdown README.md
# Project Title
This is the content of README.md
` + "```" + `

` + "```" + `python main.py
def hello():
    print("Hello World")
` + "```" + `

Rules:
1. Use code blocks with three backticks
2. After backticks, put the language/format followed by a SPACE, then the file path
3. Put complete file content on the next line
4. Close with three backticks
5. One code block per file
6. File paths are relative to repository root

This format is critical for automatic processing.`, language, context, task, language)

	// Try structured output first, fallback to regular message if model doesn't support it
	return ca.SendMessageWithStructuredOutput(conversationHistory, systemPrompt, true)
}

// ReviewFeedback processes review feedback and generates updated code
func (ca *ClaudeAgent) ReviewFeedback(feedback string, previousCode string, conversationHistory []AgentMessage) (string, TokenUsage, error) {
	systemPrompt := `You are an expert software engineer responding to code review feedback.
Your job is to:
1. Understand the feedback
2. Make the necessary changes
3. Explain what you changed and why

Be professional and collaborative.`

	userMessage := fmt.Sprintf(`Here's the review feedback on the code:

%s

Previous code:
%s

Please update the code based on this feedback.`, feedback, previousCode)

	// Add the new message to the conversation history
	updatedHistory := append(conversationHistory, AgentMessage{
		Role:    "user",
		Content: userMessage,
	})

	return ca.SendMessage(updatedHistory, systemPrompt)
}
