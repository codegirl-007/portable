# portable

Keep coding on your laptop. Run your AI agent and heavy commands on a fast
remote Linux machine. Your project folder stays on your laptop and stays in sync
automatically.

## Before you start

Install these once on your laptop:

1. [Go](https://go.dev/dl/) (to build portable)
2. [Sprite CLI](https://fly.io/sprites/), then run `sprite login`
3. [Mutagen](https://github.com/mutagen-io/mutagen/releases): put `mutagen` on
   your PATH and follow the release notes for the agent bundle

Build portable:

```sh
git clone https://github.com/codegirl-007/portable.git
cd portable
go build -o ~/.local/bin/portable .
```

Make sure `~/.local/bin` is on your PATH.

## Set up once (your machine)

`portable setup` is about **your** dev environment, not a specific repo.

```sh
portable setup
```

You will choose:

- **Coding agent**: OpenCode, Claude Code, OpenAI Codex, or Cursor Agent
- **Personal extras**: Neovim, GitHub CLI, dotfiles (optional)

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

There is no separate `portable install` command. After sync, **`portable up`**
runs on the remote workspace, in order:

1. **tools**: install runtimes/CLIs (you give `command` + `install`; portable
   runs the install only when `command -v` fails)
2. **ensure**: other prep (env files, corepack, shell profile tweaks)
3. **install**: project dependency commands in the synced repo directory

That runs on every `up`. Use idempotent lines for **ensure** and **install**
(`pnpm install`, `test -f .env || cp …`, and so on).

Add `.portable.yaml` in the project root.

**Bun + Rust** (typical full-stack repo):

```yaml
tools:
  - command: bun
    install: curl -fsSL https://bun.sh/install | bash
  - command: cargo
    install: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y

ensure:
  - grep -q '.cargo/env' "$HOME/.bashrc" 2>/dev/null || echo '. "$HOME/.cargo/env"' >> "$HOME/.bashrc"

install:
  - bun install
  - cargo fetch
```

**pnpm + Rust** (Node via apt, pnpm via Corepack):

```yaml
tools:
  - command: node
    install: curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y -qq nodejs
  - command: cargo
    install: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y

ensure:
  - corepack enable
  - grep -q '.cargo/env' "$HOME/.bashrc" 2>/dev/null || echo '. "$HOME/.cargo/env"' >> "$HOME/.bashrc"

install:
  - pnpm install
  - cargo fetch
```

**Tools only** (runtime on the workspace, no project install step yet):

```yaml
tools:
  - command: go
    install: sudo apt-get update -qq && sudo apt-get install -y -qq golang-go
```

If there is no `.portable.yaml`, portable guesses from files like `go.mod` and
`package.json` during **`portable up`**. Once you add a project file, **you**
define tools, ensure, and install; portable does not second-guess the stack.

## Change your mind later

- Different agent or dotfiles: `portable setup` again
- Different project tools: edit `.portable.yaml`, then `portable up`
- Re-run workspace bootstrap: `portable up --reprovision`
