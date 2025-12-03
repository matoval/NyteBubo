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
// If model is empty, defaults to "openrouter/auto" for automatic model selection
func NewClaudeAgent(apiKey, model string) *ClaudeAgent {
	if model == "" {
		model = "openrouter/auto" // Let OpenRouter pick the best model for each request
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
	Model      string              `json:"model"`
	Messages   []openRouterMessage `json:"messages"`
	MaxTokens  int                 `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
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

// SendMessage sends a message to OpenRouter and gets a response with usage tracking
func (ca *ClaudeAgent) SendMessage(messages []AgentMessage, systemPrompt string) (string, TokenUsage, error) {
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
func (ca *ClaudeAgent) GenerateCode(task, context, language string, conversationHistory []AgentMessage) (string, TokenUsage, error) {
	systemPrompt := fmt.Sprintf(`You are an expert software engineer working on a GitHub issue.
You have full access to the repository and need to implement the requested changes.

Programming Language: %s
Repository Context: %s

Your task: %s

Provide:
1. The specific code changes needed
2. File paths where changes should be made
3. Clear explanations of your approach

Format your response with clear sections for each file that needs to be modified.`, language, context, task)

	return ca.SendMessage(conversationHistory, systemPrompt)
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
