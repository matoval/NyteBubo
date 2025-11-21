package core

import (
	"context"
	"fmt"
	"log"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// TokenUsage tracks Claude API token usage
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	EstimatedCost float64
}

// ClaudeAgent wraps the Anthropic Claude API client
type ClaudeAgent struct {
	client *anthropic.Client
	ctx    context.Context
}

// NewClaudeAgent creates a new Claude API client
func NewClaudeAgent(apiKey string) *ClaudeAgent {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &ClaudeAgent{
		client: &client,
		ctx:    context.Background(),
	}
}

// AgentMessage represents a message in the conversation
type AgentMessage struct {
	Role    string
	Content string
}

// SendMessage sends a message to Claude and gets a response with usage tracking
func (ca *ClaudeAgent) SendMessage(messages []AgentMessage, systemPrompt string) (string, TokenUsage, error) {
	// Convert our messages to the SDK format
	var apiMessages []anthropic.MessageParam
	for _, msg := range messages {
		var role anthropic.MessageParamRole
		switch msg.Role {
		case "user":
			role = anthropic.MessageParamRoleUser
		case "assistant":
			role = anthropic.MessageParamRoleAssistant
		default:
			return "", TokenUsage{}, fmt.Errorf("invalid role: %s", msg.Role)
		}

		apiMessages = append(apiMessages, anthropic.MessageParam{
			Role: role,
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: msg.Content},
			}},
		})
	}

	// Create message params
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		MaxTokens: 8096,
		Messages:  apiMessages,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
	}

	// Send the message
	message, err := ca.client.Messages.New(ca.ctx, params)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("failed to send message: %w", err)
	}

	// Track token usage
	usage := TokenUsage{
		InputTokens:   message.Usage.InputTokens,
		OutputTokens:  message.Usage.OutputTokens,
		TotalTokens:   message.Usage.InputTokens + message.Usage.OutputTokens,
		EstimatedCost: calculateCost(message.Usage.InputTokens, message.Usage.OutputTokens),
	}

	// Log usage information
	log.Printf("📊 Claude API - Input: %d | Output: %d | Total: %d tokens | Cost: $%.4f",
		usage.InputTokens, usage.OutputTokens, usage.TotalTokens, usage.EstimatedCost)

	// Extract the response text
	if len(message.Content) == 0 {
		return "", usage, fmt.Errorf("no content in response")
	}

	// Get the text from the first content block
	contentBlock := message.Content[0]
	if contentBlock.Type == "text" && contentBlock.Text != "" {
		return contentBlock.Text, usage, nil
	}

	return "", usage, fmt.Errorf("unexpected content type: %s", contentBlock.Type)
}

// calculateCost estimates the cost based on Claude 3.7 Sonnet pricing
// As of January 2025: $3 per million input tokens, $15 per million output tokens
func calculateCost(inputTokens, outputTokens int64) float64 {
	const (
		inputCostPerMillion  = 3.0
		outputCostPerMillion = 15.0
	)

	inputCost := float64(inputTokens) / 1000000.0 * inputCostPerMillion
	outputCost := float64(outputTokens) / 1000000.0 * outputCostPerMillion

	return inputCost + outputCost
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
