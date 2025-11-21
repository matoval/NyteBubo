# NyteBubo

An AI-powered GitHub agent that runs on your local server, automatically analyzing assigned issues and creating pull requests using Claude AI.

## Overview

NyteBubo is a focused polling agent that monitors GitHub repositories for issue assignments. When assigned to an issue, it:

1. **Analyzes Issues**: Reads and understands what needs to be done
2. **Asks Questions**: Comments on the issue if anything is unclear
3. **Creates PRs**: Automatically creates a pull request with the implementation
4. **Responds to Feedback**: Updates code based on PR review comments

## Key Features

- **No Public Endpoint Required**: Runs entirely on your local/home server using polling
- **Polling Architecture**: Checks for assigned issues every 30 seconds (configurable)
- **Claude AI Integration**: Uses Anthropic's Claude for intelligent code generation
- **Token Usage Tracking**: Tracks API usage and costs per issue
- **Cost Estimation**: Real-time cost estimates based on Claude 3.7 Sonnet pricing
- **Export to CSV**: Export usage statistics for analysis
- **Persistent State Management**: SQLite database tracks conversation history and usage
- **Simple Configuration**: YAML config and environment variables
- **Optional Webhook Mode**: Can also run as webhook server if needed

## Prerequisites

- Go 1.22 or higher
- GitHub account with repository access
- Anthropic Claude API key
- GitHub Personal Access Token with repo permissions

## Installation

### Build from Source

```bash
git clone https://github.com/matoval/NyteBubo.git
cd NyteBubo
go build -o nyte-bubo
```

### Install Binary

```bash
go install github.com/matoval/NyteBubo@latest
```

## Quick Start

### 1. Create Configuration

Create a `config.yaml` file:

```bash
./nyte-bubo init
```

Edit `config.yaml` and add your repositories:

```yaml
working_dir: "./workspace"
state_db_path: "./agent_state.db"

# Polling configuration
poll_interval: 30  # Check every 30 seconds
repositories:
  - "yourusername/your-repo"
  - "yourorg/another-repo"
```

### 2. Set Environment Variables

For security, set your API credentials as environment variables:

```bash
export CLAUDE_API_KEY="your-claude-api-key"
export GITHUB_TOKEN="your-github-personal-access-token"
```

### 3. Start the Agent

```bash
./nyte-bubo agent
```

The agent will start polling the specified repositories every 30 seconds for assigned issues.

That's it! No public endpoint, no webhooks to configure. The agent runs entirely on your local network.

### 4. View Usage Statistics

Check token usage and costs for resolved issues:

```bash
# View stats in terminal
./nyte-bubo stats

# Export to CSV
./nyte-bubo stats --export --file usage_stats.csv
```

## How It Works

### Polling Architecture

NyteBubo uses a polling approach instead of webhooks:

1. **Continuous Monitoring**: The agent checks your configured repositories every 30 seconds (or your configured interval)
2. **Issue Detection**: When it finds an issue assigned to the bot's GitHub account, it processes it
3. **State Tracking**: Uses SQLite to remember which issues have been processed and their status
4. **No Public Endpoint**: Runs entirely on your local network - perfect for home servers

### Workflow

1. **Issue Assignment**:
   - Assign the bot's GitHub account to an issue
   - On next poll cycle, the agent detects the assignment
   - Claude analyzes the issue and posts a comment with its understanding
   - If anything is unclear, it asks clarifying questions

2. **Clarification** (if needed):
   - You respond to the agent's questions in the issue comments
   - The agent detects new comments on next poll
   - Processes your responses and asks follow-ups if needed
   - This continues until the agent has a clear understanding

3. **Implementation**:
   - Once ready, the agent creates a new branch
   - Claude generates the necessary code changes
   - The agent creates a pull request with the implementation
   - Links the PR to the original issue

4. **Code Review**:
   - You review the PR and leave comments or suggestions
   - The agent detects new PR comments on next poll
   - Claude generates updated code based on your comments
   - The agent pushes the changes to the PR branch

### Architecture

```
┌─────────────────────────────────┐
│  NyteBubo Polling Agent          │
│                                  │
│  ┌─────────────────────────┐    │
│  │  Poller (every 30s)     │    │
│  │  - Checks assigned      │    │
│  │    issues               │    │
│  └───────────┬─────────────┘    │
│              v                   │
│  ┌─────────────────────────┐    │
│  │  Issue Agent Workflow   │    │
│  │  - Analyze              │    │
│  │  - Generate code        │    │
│  │  - Create PRs           │    │
│  └───────────┬─────────────┘    │
│              v                   │
│  ┌──────────────┬──────────┐    │
│  │ GitHub API   │ Claude AI│    │
│  │ Client       │ Client   │    │
│  └──────────────┴──────────┘    │
│              v                   │
│  ┌─────────────────────────┐    │
│  │  State Manager (SQLite) │    │
│  │  - Track conversations  │    │
│  │  - Prevent reprocessing │    │
│  └─────────────────────────┘    │
└────────┬────────────────────────┘
         │ Polls via GitHub API
         v
   ┌──────────┐
   │  GitHub  │
   │  Repos   │
   └──────────┘
```

## Project Structure

```
NyteBubo/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command
│   ├── init.go            # Init command
│   └── agent.go           # Agent server command
├── internal/
│   ├── cli/               # CLI logic
│   │   └── init.go        # Config file generation
│   ├── core/              # Core functionality
│   │   ├── github.go      # GitHub API client
│   │   ├── claude.go      # Claude AI client
│   │   └── state.go       # State management
│   ├── types/             # Type definitions
│   │   └── config.go      # Configuration types
│   ├── templates/         # Templates
│   │   └── init.tmpl      # Config template
│   └── workflows/         # Agent workflows
│       └── issue_to_pr.go # Issue-to-PR workflow
├── server/                # Server implementation
│   └── webhook.go        # Webhook handlers
├── main.go               # Application entry point
├── config.yaml           # Configuration file (optional)
├── go.mod                # Go module definition
└── README.md             # This file
```

## Configuration

### config.yaml

```yaml
# Working directories
working_dir: "./workspace"
state_db_path: "./agent_state.db"

# Polling configuration (default mode)
poll_interval: 30  # Check for new issues every 30 seconds
repositories:
  - "owner/repo"  # Add your repositories
  - "owner/another-repo"

# Optional: Set credentials here (not recommended - use env vars instead)
# claude_api_key: ""
# github_token: ""

# Optional: Webhook mode (requires public endpoint)
# webhook_mode: true
# server_port: 8080
# webhook_secret: "your-secret"
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `CLAUDE_API_KEY` | Your Anthropic Claude API key | Yes |
| `GITHUB_TOKEN` | GitHub Personal Access Token with repo access | Yes |

### Webhook Mode (Optional)

If you have a public endpoint and prefer webhook mode:

1. Set `webhook_mode: true` in config.yaml
2. Configure `server_port` and `webhook_secret`
3. Set up GitHub webhook pointing to your endpoint
4. See original webhook documentation for setup details

Polling mode is recommended for home servers.

## Token Usage Tracking

NyteBubo automatically tracks Claude API token usage and costs for every issue it processes.

### Real-time Logging

During operation, the agent logs token usage for each API call:

```
📊 Claude API - Input: 1,245 | Output: 856 | Total: 2,101 tokens | Cost: $0.0162
```

### Usage Statistics Command

View comprehensive statistics for all processed issues:

```bash
./nyte-bubo stats
```

Example output:

```
╔═══════════════════════════════════════════════════════════════════════╗
║                     Token Usage Statistics                             ║
╚═══════════════════════════════════════════════════════════════════════╝

Issue                          Input Tokens  Output Tokens  Cost        Status
────────────────────────────────────────────────────────────────────────────
owner/repo#42                          5234           3421  $ 0.0671    pr_created
owner/repo#43                          3892           2156  $ 0.0441    reviewing
────────────────────────────────────────────────────────────────────────────
TOTAL                                  9126           5577  $ 0.1112

📊 Summary:
  Total Issues: 2
  Total Tokens: 9126 (input) + 5577 (output) = 14703 total
  Total Cost: $0.1112
  Average Cost per Issue: $0.0556
```

### Export to CSV

Export statistics to a CSV file for further analysis:

```bash
./nyte-bubo stats --export --file usage_stats.csv
```

The CSV includes:
- Repository and issue details
- Token counts (input, output, total)
- Estimated costs
- Issue status and timestamps

### Cost Estimation

Costs are estimated based on Claude 3.7 Sonnet pricing (as of January 2025):
- **Input tokens**: $3.00 per million tokens
- **Output tokens**: $15.00 per million tokens

**Note**: These are estimates. Actual costs may vary. Check your Anthropic billing for precise charges.

## Security Best Practices

1. **Never commit credentials**: Use environment variables for API keys and tokens
2. **Set webhook secret**: Always configure a webhook secret to verify GitHub events
3. **Use HTTPS**: In production, use HTTPS for your webhook endpoint
4. **Restrict bot permissions**: Create a dedicated GitHub user for the bot with minimal required permissions
5. **Review PRs carefully**: Always review agent-generated code before merging

## API Usage

### GitHub API

The agent uses the following GitHub API endpoints:
- Get issue details
- Create and list issue comments
- Create branches and pull requests
- Get and update file contents
- List PR comments

### Claude AI API

The agent uses Claude 3.7 Sonnet Latest model with:
- Conversational message API
- System prompts for context
- Multi-turn conversations for clarification
- Code generation capabilities

## Troubleshooting

### Agent not detecting assigned issues

1. Check that repositories are correctly configured in `config.yaml`
2. Verify the GitHub token has read access to the repositories
3. Ensure issues are assigned to the GitHub account associated with the token
4. Check agent logs for polling activity and errors
5. Verify poll interval is reasonable (30s recommended)

### Agent not responding to issues

1. Check server logs for errors
2. Verify API credentials are set correctly
3. Check `agent_state.db` for conversation state
4. Ensure Claude API key has sufficient quota

### Build errors

```bash
# Clean and rebuild
go clean
go mod tidy
go build -o nyte-bubo
```

## Development

### Running Tests

```bash
go test ./...
```

### Adding New Workflows

1. Create a new file in `internal/workflows/`
2. Implement the workflow interface
3. Add webhook handlers in `server/webhook.go`
4. Update the agent command in `cmd/agent.go`

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[Add your license here]

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI
- Powered by [Claude AI](https://www.anthropic.com/claude) from Anthropic
- GitHub integration via [go-github](https://github.com/google/go-github)
- State management with [SQLite](https://modernc.org/sqlite)
