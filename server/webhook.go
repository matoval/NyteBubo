package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"NyteBubo/internal/workflows"

	"github.com/google/go-github/v63/github"
)

// WebhookServer handles GitHub webhook events
type WebhookServer struct {
	agent         *workflows.IssueAgent
	webhookSecret string
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(agent *workflows.IssueAgent, webhookSecret string) *WebhookServer {
	return &WebhookServer{
		agent:         agent,
		webhookSecret: webhookSecret,
	}
}

// HandleWebhook processes incoming GitHub webhook events
func (ws *WebhookServer) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify webhook signature
	if ws.webhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !ws.verifySignature(signature, body) {
			log.Println("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Get the event type
	eventType := r.Header.Get("X-GitHub-Event")
	log.Printf("Received GitHub event: %s", eventType)

	// Handle different event types
	switch eventType {
	case "issues":
		ws.handleIssuesEvent(body, w)
	case "issue_comment":
		ws.handleIssueCommentEvent(body, w)
	case "pull_request_review_comment":
		ws.handlePRCommentEvent(body, w)
	case "ping":
		log.Println("Received ping event")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "pong"}`))
	default:
		log.Printf("Unhandled event type: %s", eventType)
		w.WriteHeader(http.StatusOK)
	}
}

// verifySignature verifies the GitHub webhook signature
func (ws *WebhookServer) verifySignature(signature string, body []byte) bool {
	if signature == "" {
		return false
	}

	// Extract the hash from the signature (format: sha256=hash)
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(ws.webhookSecret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	return hmac.Equal([]byte(parts[1]), []byte(expectedMAC))
}

// handleIssuesEvent handles issue events (opened, assigned, etc.)
func (ws *WebhookServer) handleIssuesEvent(body []byte, w http.ResponseWriter) {
	var event github.IssuesEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error parsing issues event: %v", err)
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	action := event.GetAction()
	log.Printf("Issues event action: %s", action)

	// Only handle "assigned" events where the bot is assigned
	if action == "assigned" {
		owner := event.Repo.Owner.GetLogin()
		repo := event.Repo.GetName()
		issueNumber := event.Issue.GetNumber()

		log.Printf("Agent assigned to issue #%d in %s/%s", issueNumber, owner, repo)

		// Handle the assignment asynchronously
		go func() {
			if err := ws.agent.HandleIssueAssignment(owner, repo, issueNumber); err != nil {
				log.Printf("Error handling issue assignment: %v", err)
			}
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Processing issue assignment"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleIssueCommentEvent handles issue comment events
func (ws *WebhookServer) handleIssueCommentEvent(body []byte, w http.ResponseWriter) {
	var event github.IssueCommentEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error parsing issue comment event: %v", err)
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	action := event.GetAction()
	log.Printf("Issue comment event action: %s", action)

	// Only handle "created" comments
	if action == "created" {
		owner := event.Repo.Owner.GetLogin()
		repo := event.Repo.GetName()
		issueNumber := event.Issue.GetNumber()
		commentBody := event.Comment.GetBody()
		commentAuthor := event.Comment.User.GetLogin()

		// Ignore comments from the bot itself (to avoid infinite loops)
		if strings.Contains(strings.ToLower(commentAuthor), "bot") {
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Printf("New comment on issue #%d in %s/%s", issueNumber, owner, repo)

		// Handle the comment asynchronously
		go func() {
			if err := ws.agent.HandleIssueComment(owner, repo, issueNumber, commentBody); err != nil {
				log.Printf("Error handling issue comment: %v", err)
			}
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Processing comment"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handlePRCommentEvent handles pull request review comment events
func (ws *WebhookServer) handlePRCommentEvent(body []byte, w http.ResponseWriter) {
	var event github.PullRequestReviewCommentEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error parsing PR comment event: %v", err)
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	action := event.GetAction()
	log.Printf("PR comment event action: %s", action)

	// Only handle "created" comments
	if action == "created" {
		owner := event.Repo.Owner.GetLogin()
		repo := event.Repo.GetName()
		prNumber := event.PullRequest.GetNumber()
		commentBody := event.Comment.GetBody()
		commentAuthor := event.Comment.User.GetLogin()

		// Ignore comments from the bot itself
		if strings.Contains(strings.ToLower(commentAuthor), "bot") {
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Printf("New comment on PR #%d in %s/%s", prNumber, owner, repo)

		// Handle the comment asynchronously
		go func() {
			if err := ws.agent.HandlePRComment(owner, repo, prNumber, commentBody); err != nil {
				log.Printf("Error handling PR comment: %v", err)
			}
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Processing PR comment"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Start starts the webhook server
func (ws *WebhookServer) Start(port int) error {
	http.HandleFunc("/webhook", ws.HandleWebhook)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting webhook server on %s", addr)
	return http.ListenAndServe(addr, nil)
}
