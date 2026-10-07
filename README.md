# portable

Keep coding on your laptop. Run your AI agent and heavy commands on a fast
remote Linux machine. Your project folder stays on your laptop and stays in sync
automatically.

## Before you start

Install these once on your laptop:

1. [Go](https://go.dev/dl/) (to build portable)
2. [Sprite CLI](https://fly.io/sprites/) — then run `sprite login`
3. [Mutagen](https://github.com/mutagen-io/mutagen/releases) — put `mutagen` on
   your PATH and follow the release notes for the agent bundle

Build portable:

```sh
git clone https://github.com/codegirl-007/portable.git
cd portable
go build -o ~/.local/bin/portable .
```

Make sure `~/.local/bin` is on your PATH.

## Set up once (your machine)

`portable setup` is about **your** dev environment — not about a specific repo.

```sh
portable setup
```

You will choose:

- **Coding agent** — OpenCode, Claude Code, OpenAI Codex, or Cursor Agent
- **Personal extras** — Neovim, GitHub CLI, dotfiles (optional)

That saves to `~/.config/portable/config.toml` and applies to every project.

Set the API key for your agent on your laptop before the first `portable up`
(if the agent needs one):

| Agent | Environment variable |
| --- | --- |
| OpenCode | `OPENCODE_API_KEY` (or use OpenCode locally) |
| Claude Code | `ANTHROPIC_API_KEY` |
| OpenAI Codex | `OPENAI_API_KEY` |
| Cursor Agent | `CURSOR_API_KEY` |

## Use with a project

```sh
cd ~/projects/my-app
portable up
```

First time takes a few minutes. After that, `portable up` is usually quick.

**Start your agent:**

```sh
portable agent
```

**Run anything else on the remote machine:**

```sh
portable run go test ./...
portable ssh
```

**When you're done for the day:**

```sh
portable down
```

**Remove the remote machine completely:**

```sh
portable destroy
```

## Per-project: tools this repo needs

Each project can list **runtimes and CLIs** the workspace should have, plus
**install commands** to run in the repo after sync. Add `.portable.yaml` in the
project root:

```yaml
# This project uses Bun and Rust on the workspace.
tools:
  - bun
  - rust

# Run in the project directory after each portable up.
install:
  - bun install
  - cargo fetch
```

Supported `tools` names: `bun`, `deno`, `node`, `npm`, `pnpm`, `yarn`, `go`,
`rust`.

Use `ensure` when you need a one-time prep step that is not a built-in `tools`
name — for example enabling Corepack or creating a local env file before
`pnpm install`:

```yaml
tools:
  - node

ensure:
  - corepack enable
  - test -f .env || cp .env.example .env

install:
  - pnpm install
```

If there is no `.portable.yaml`, portable guesses from files like `go.mod` and
`package.json`. Once you add a project file, **you** define tools and install —
portable does not second-guess the stack.

## Change your mind later

- Different agent or dotfiles → `portable setup` again
- Different project tools → edit `.portable.yaml`, then `portable up`
- Re-run workspace bootstrap → `portable up --reprovision`
