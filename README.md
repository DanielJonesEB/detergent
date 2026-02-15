# Detergent

Deterministic agent invocation. Define a chain of agent calls that will get invoked, in sequence, on every commit. Get alerted via the Claude Code statusline when downstream agents make changes, and use the `/detergent-rebase` skill to automatically pull them in.

Kinda like CI, but local.

Everything is in Git, so you lose nothing. If you _also_ use [`claudit`](https://github.com/re-cinq/claudit), your agent can automatically attach chat history as Git Notes.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/re-cinq/detergent/master/scripts/install.sh | bash
```

Then, in your repo:

```bash
detergent init # Set up skills
detergent run # Start the daemon (there's a skill for this too)
detergent status # See what's going on
detergent status -f -n 1 # Follow the status, refreshing every 1 second
```

## Quick Start

Create a config file `detergent.yaml`:

```yaml
agent:
  command: claude
  args: ["--dangerously-skip-permissions", "-p"]

settings:
  poll_interval: 5s
  watches: main

concerns:
  - name: security
    prompt: "Review for security vulnerabilities. Fix any issues found."

  - name: docs
    prompt: "Ensure public functions have clear documentation."

  - name: style
    prompt: "Fix any code style issues."
```

Concerns are processed as an ordered chain: each concern watches the one before it, and the first concern watches the branch specified in `settings.watches` (defaults to `main`).

### Permissions

If your agent is Claude Code, you can pre-approve tool permissions instead of using `--dangerously-skip-permissions`. Add an optional `permissions` block — detergent writes it as `.claude/settings.json` in each worktree before invoking the agent:

```yaml
permissions:
  allow:
    - Edit
    - Write
    - "Bash(*)"
```

## Why?

Models will absolutely forget things, especially if context is overloaded (there's too much) or polluted (too many different topics). However, if you prompt them with a clear context, they'll spot what they overlooked straight away.

### Why not...

* **...do it yourself?** As a human, trying to remember to run the same set of quality-check prompts before every commit is a hassle.
* **...do it in CI?** Leaving these tasks until CI delays feedback, your agent might not be configured to read from CI, and sometimes you don't want to push.
* **...use a Git hook?** Not being able to commit in a hurry would be inconvenient. Plus, with `detergent` you can tell your main agent to commit with `skip detergent` if you want.

## Usage

```bash
# Validate your config (defaults to detergent.yaml)
detergent validate

# See the concern chain
detergent viz

# Run once and exit
detergent run --once

# Run as daemon (polls for changes)
detergent run

# Check status of each concern
detergent status

# Live-updating status (like watch, tails active agent logs)
detergent status -f

# View agent logs for a concern
detergent logs security

# Follow agent logs in real-time
detergent logs -f security

# Use a different config file
detergent run --path my-config.yaml

# Initialize Claude Code integration (statusline + skills)
detergent init
```

## How It Works

1. Detergent watches branches for new commits
2. When a commit arrives, it creates a worktree for each triggered concern
3. The agent receives: the prompt + upstream commit messages + diffs
4. Agent changes are committed with `[CONCERN]` tags and `Triggered-By:` trailers
5. If no changes needed, a git note records the review
6. Downstream concerns see upstream commits and can build on them

**Note:** When running as a daemon, detergent automatically reloads `detergent.yaml` at the start of each poll cycle. Config changes take effect immediately without requiring a restart.

## Claude Code Integration

`detergent init` sets up:

- **Statusline** — shows the concern pipeline in Claude Code's status bar:
  ```
  main ─── security ✓ ── docs ⟳ ── style ·
  ```
  - When on a terminal concern branch that's behind HEAD, displays a bold yellow warning: `⚠ use /rebase <branch> to pick up latest changes`
- **Skills** — adds `/detergent-start` to start the daemon as a background task and `/detergent-rebase` for rebasing concern branch changes onto their upstream

### Statusline Symbols

| Symbol | Meaning |
|--------|---------|
| `◎` | Change detected |
| `⟳` | Agent running / committing |
| `◯` | Pending (behind HEAD) |
| `✗` | Failed |
| `⊘` | Skipped |
| `*` | Done, produced modifications |
| `✓` | Done, no changes needed |
| `·` | Never run |

## Git Conventions

- **Branches**: `detergent/{concern-name}` (configurable prefix)
- **Commits**: `[SECURITY] Fix SQL injection in login` with `Triggered-By: abc123` trailer
- **Notes**: `[SECURITY] Reviewed, no changes needed` when agent makes no changes
- **Skipping processing**: Add `[skip ci]`, `[ci skip]`, `[skip detergent]`, or `[detergent skip]` to commit messages to prevent detergent from processing them

## Development

```bash
make build    # Build binary (bin/detergent)
make test     # Run acceptance tests
make lint     # Run linter (requires golangci-lint)
make fmt      # Format code
```

## License

[AI Native Application License (AINAL) v2.0](LICENSE) ([source](https://github.com/re-cinq/ai-native-application-license))
