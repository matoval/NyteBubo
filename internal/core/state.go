package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// State represents the conversation state for an issue
type State struct {
	ID              int64
	Owner           string
	Repo            string
	IssueNumber     int
	Status          string // "analyzing", "waiting_for_clarification", "ready_to_implement", "implementing", "pr_created", "reviewing", "completed"
	PRNumber        *int
	BranchName      string
	Conversation    []AgentMessage
	// Token usage tracking
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCost         float64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CompletedAt       *time.Time
}

// StateManager handles persistence of agent state
type StateManager struct {
	db *sql.DB
}

// NewStateManager creates a new state manager
func NewStateManager(dbPath string) (*StateManager, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &StateManager{db: db}, nil
}

// createTables creates the necessary database tables
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS agent_states (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner TEXT NOT NULL,
		repo TEXT NOT NULL,
		issue_number INTEGER NOT NULL,
		status TEXT NOT NULL,
		pr_number INTEGER,
		branch_name TEXT,
		conversation TEXT,
		total_input_tokens INTEGER DEFAULT 0,
		total_output_tokens INTEGER DEFAULT 0,
		total_cost REAL DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		completed_at DATETIME,
		UNIQUE(owner, repo, issue_number)
	);

	CREATE INDEX IF NOT EXISTS idx_states_lookup
	ON agent_states(owner, repo, issue_number);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

// GetState retrieves the state for a specific issue
func (sm *StateManager) GetState(owner, repo string, issueNumber int) (*State, error) {
	query := `
		SELECT id, owner, repo, issue_number, status, pr_number, branch_name,
		       conversation, total_input_tokens, total_output_tokens, total_cost,
		       created_at, updated_at, completed_at
		FROM agent_states
		WHERE owner = ? AND repo = ? AND issue_number = ?
	`

	var state State
	var conversationJSON string
	var prNumber sql.NullInt64
	var completedAt sql.NullTime

	err := sm.db.QueryRow(query, owner, repo, issueNumber).Scan(
		&state.ID,
		&state.Owner,
		&state.Repo,
		&state.IssueNumber,
		&state.Status,
		&prNumber,
		&state.BranchName,
		&conversationJSON,
		&state.TotalInputTokens,
		&state.TotalOutputTokens,
		&state.TotalCost,
		&state.CreatedAt,
		&state.UpdatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No state found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	if prNumber.Valid {
		prNum := int(prNumber.Int64)
		state.PRNumber = &prNum
	}

	if completedAt.Valid {
		state.CompletedAt = &completedAt.Time
	}

	// Unmarshal conversation
	if conversationJSON != "" {
		if err := json.Unmarshal([]byte(conversationJSON), &state.Conversation); err != nil {
			return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
		}
	}

	return &state, nil
}

// SaveState saves or updates the state for an issue
func (sm *StateManager) SaveState(state *State) error {
	// Marshal conversation to JSON
	conversationJSON, err := json.Marshal(state.Conversation)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	now := time.Now()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	state.UpdatedAt = now

	query := `
		INSERT INTO agent_states (owner, repo, issue_number, status, pr_number, branch_name, conversation,
		                          total_input_tokens, total_output_tokens, total_cost,
		                          created_at, updated_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner, repo, issue_number) DO UPDATE SET
			status = excluded.status,
			pr_number = excluded.pr_number,
			branch_name = excluded.branch_name,
			conversation = excluded.conversation,
			total_input_tokens = excluded.total_input_tokens,
			total_output_tokens = excluded.total_output_tokens,
			total_cost = excluded.total_cost,
			updated_at = excluded.updated_at,
			completed_at = excluded.completed_at
	`

	result, err := sm.db.Exec(
		query,
		state.Owner,
		state.Repo,
		state.IssueNumber,
		state.Status,
		state.PRNumber,
		state.BranchName,
		string(conversationJSON),
		state.TotalInputTokens,
		state.TotalOutputTokens,
		state.TotalCost,
		state.CreatedAt,
		state.UpdatedAt,
		state.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	if state.ID == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert id: %w", err)
		}
		state.ID = id
	}

	return nil
}

// DeleteState removes the state for an issue
func (sm *StateManager) DeleteState(owner, repo string, issueNumber int) error {
	query := `DELETE FROM agent_states WHERE owner = ? AND repo = ? AND issue_number = ?`
	_, err := sm.db.Exec(query, owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}
	return nil
}

// GetAllIssuesWithStats retrieves all issues with their usage stats
func (sm *StateManager) GetAllIssuesWithStats() ([]State, error) {
	query := `
		SELECT id, owner, repo, issue_number, status, pr_number, branch_name,
		       conversation, total_input_tokens, total_output_tokens, total_cost,
		       created_at, updated_at, completed_at
		FROM agent_states
		ORDER BY created_at DESC
	`

	rows, err := sm.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query states: %w", err)
	}
	defer rows.Close()

	var states []State
	for rows.Next() {
		var state State
		var conversationJSON string
		var prNumber sql.NullInt64
		var completedAt sql.NullTime

		err := rows.Scan(
			&state.ID,
			&state.Owner,
			&state.Repo,
			&state.IssueNumber,
			&state.Status,
			&prNumber,
			&state.BranchName,
			&conversationJSON,
			&state.TotalInputTokens,
			&state.TotalOutputTokens,
			&state.TotalCost,
			&state.CreatedAt,
			&state.UpdatedAt,
			&completedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if prNumber.Valid {
			prNum := int(prNumber.Int64)
			state.PRNumber = &prNum
		}

		if completedAt.Valid {
			state.CompletedAt = &completedAt.Time
		}

		// Unmarshal conversation
		if conversationJSON != "" {
			if err := json.Unmarshal([]byte(conversationJSON), &state.Conversation); err != nil {
				return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
			}
		}

		states = append(states, state)
	}

	return states, nil
}

// Close closes the database connection
func (sm *StateManager) Close() error {
	return sm.db.Close()
}
