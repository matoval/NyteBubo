# NyteBubo Setup Guide

This guide will walk you through setting up NyteBubo to run on your local/home server with polling mode (no public endpoint required).

## Prerequisites

Before you begin, make sure you have:

1. **Go 1.22+** installed
2. **GitHub repositories** you want to monitor
3. **An OpenRouter API account** with credits
4. **A local server** (home server, VPS, or even your laptop)

## Step 1: Get Your API Credentials

### OpenRouter API Key

1. Go to https://openrouter.ai/keys
2. Sign in or create an account
3. Click "Create Key"
4. Give it a name (e.g., "NyteBubo Agent")
5. Copy and save the API key securely

### GitHub Personal Access Token

1. Go to https://github.com/settings/tokens
2. Click "Generate new token" > "Generate new token (classic)"
3. Give it a descriptive name (e.g., "NyteBubo Agent")
4. Select the following scopes:
   - `repo` (Full control of private repositories)
   - Optionally: `workflow` if you want the bot to trigger workflows
5. Click "Generate token"
6. Copy and save the token securely (you won't be able to see it again!)

### Webhook Secret (Recommended)

Generate a random secret for webhook verification:

```bash
openssl rand -hex 32
```

Copy and save this secret.

## Step 2: Build NyteBubo

```bash
# Clone the repository
git clone https://github.com/yourusername/NyteBubo.git
cd NyteBubo

# Install dependencies
go mod download

# Build the binary
go build -o nyte-bubo

# Verify the build
./nyte-bubo --help
```

## Step 3: Create Configuration

Create and configure your `config.yaml`:

```bash
./nyte-bubo init
```

Edit the generated `config.yaml` and add your repositories:

```yaml
working_dir: "./workspace"
state_db_path: "./agent_state.db"

# Polling configuration
poll_interval: 30  # Check every 30 seconds (adjust as needed)
repositories:
  - "yourusername/your-repo"
  - "yourorg/another-repo"
  # Add all repositories you want to monitor

# AI Model (optional - defaults to qwen/qwen3-coder:free)
openrouter_model: "qwen/qwen3-coder:free"

# Optional: Set credentials here (not recommended)
# openrouter_api_key: ""
# github_token: ""
```

**Important**: Replace the example repositories with your actual GitHub repositories in the format `owner/repo`.

## Step 4: Set Up Environment Variables

### Option A: Using .env file (Development)

1. Copy the example file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and add your credentials:
   ```bash
   OPENROUTER_API_KEY=sk-or-v1-...
   GITHUB_TOKEN=ghp_...
   ```

3. Load the environment:
   ```bash
   source .env
   ```

### Option B: Export directly (Quick test)

```bash
export OPENROUTER_API_KEY="sk-or-v1-..."
export GITHUB_TOKEN="ghp_..."
```

### Option C: Systemd service (Production)

Create `/etc/systemd/system/nyte-bubo.service`:

```ini
[Unit]
Description=NyteBubo GitHub Polling Agent
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/NyteBubo
Environment="OPENROUTER_API_KEY=sk-or-v1-..."
Environment="GITHUB_TOKEN=ghp_..."
ExecStart=/path/to/NyteBubo/nyte-bubo agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Then:
```bash
sudo systemctl daemon-reload
sudo systemctl enable nyte-bubo
sudo systemctl start nyte-bubo
```

## Step 5: Start the Agent

```bash
./nyte-bubo agent
```

You should see:
```
╔═══════════════════════════════════════════════╗
║        NyteBubo Agent Starting (Polling)      ║
╚═══════════════════════════════════════════════╝

Configuration:
  Mode: Polling
  Poll Interval: 30s
  Repositories: [your/repos]
  Working Directory: ./workspace
  State Database: ./agent_state.db

The agent is now polling for assigned issues.
No public endpoint required - runs entirely on your local network!

Press Ctrl+C to stop the agent.
```

That's it! The agent is now running and will check for assigned issues every 30 seconds. No need to expose your server to the internet or configure webhooks.

## Step 6: Create a Bot User (Recommended)

For best practices, create a dedicated GitHub user for the bot:

1. Create a new GitHub account (e.g., `myapp-bot`)
2. Add this user as a collaborator to your repository
3. Generate a Personal Access Token for this user
4. Use this token as your `GITHUB_TOKEN`

This way:
- PRs and comments are clearly from the bot
- You can easily identify and manage bot activity
- You can revoke access without affecting your personal account

## Step 7: Test the Agent

1. Create a new issue in one of your monitored repositories
2. Assign the bot user (or yourself if using your own token) to the issue
3. Wait for the next poll cycle (up to 30 seconds)
4. Watch for a comment from the agent analyzing the issue
5. Respond to any clarifying questions
6. Wait for the agent to create a PR
7. Review the PR and leave comments
8. Watch the agent update the code based on your feedback

## Troubleshooting

### Agent not detecting assigned issues

**Check agent logs:**
```bash
# If running directly
./nyte-bubo agent

# If using systemd
journalctl -u nyte-bubo -f
```

**Common issues:**
- Repositories not correctly configured in `config.yaml` (must be `owner/repo` format)
- Issues not assigned to the bot's GitHub account
- GitHub token doesn't have read access to the repositories
- Poll interval too long (reduce it for testing)
- Network connectivity issues preventing GitHub API access

### Authentication errors

**Verify environment variables:**
```bash
echo $OPENROUTER_API_KEY
echo $GITHUB_TOKEN
```

**Check token permissions:**
- GitHub token needs `repo` scope
- OpenRouter API key needs to be valid and have credits

### Database errors

**Reset the state database:**
```bash
rm agent_state.db
./nyte-bubo agent
```

### Building errors

**Clean and rebuild:**
```bash
go clean
go mod tidy
go build -o nyte-bubo
```

## Next Steps

- Read the [README.md](README.md) for detailed information
- Monitor the agent's behavior and adjust prompts in `internal/core/openrouter.go`
- Customize workflows in `internal/workflows/issue_to_pr.go`
- Adjust poll interval in `config.yaml` based on your needs
- Adjust AI model in `config.yaml` (default: `qwen/qwen3-coder:free` - best free coding model)
- Add more repositories to monitor as needed

## Getting Help

- Check the [README.md](README.md) for common questions
- Review agent logs for error messages
- Verify your `config.yaml` configuration
- Check GitHub API rate limits
- Open an issue in the NyteBubo repository

## Security Checklist

Before going to production:

- [ ] Using environment variables (not config file) for credentials
- [ ] Using a dedicated bot user account
- [ ] Bot user has minimal required permissions (read access to repos)
- [ ] `.env` file is in `.gitignore`
- [ ] Server logs don't expose credentials
- [ ] Regular rotation of API keys and tokens
- [ ] Agent runs on secure local network
- [ ] GitHub token scope limited to necessary repositories

---

Happy coding with NyteBubo!
