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

Install portable from [GitHub Releases](https://github.com/codegirl-007/portable/releases)
(linux/darwin/windows, amd64 and arm64). Unzip or extract, put `portable` on your PATH.

Or build from source:

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

## Another branch

```sh
portable up --branch feat/foo
```

Your current checkout stays on its branch. portable adds a worktree under `.worktrees/` and starts a second remote for `feat/foo`. You get a shell in that worktree, so git is already on `feat/foo`. Checking out a branch does not start anything.

From the worktree directory, you can run any of the portable commands.

Alternatively, from the main repo (or anywhere in it):

```sh
portable agent --branch feat/foo
portable down --branch feat/foo
portable destroy --branch feat/foo
```

`destroy` kills the remote, not the worktree. Use `--no-enter` if you don't want a shell after `up`.

## From your phone (or any browser)

The agent session lives in **tmux on the Sprite**, not in your laptop terminal. After `portable up`:

```sh
portable remote
portable remote --branch feat/foo
```

That prints a URL and a QR code. Open it on your phone or another computer. You get a live terminal with extra keys for Esc, Tab, Ctrl-C, and arrows. Typing there is the same session as `portable agent` on the laptop.

From a laptop TTY, detach with **Ctrl-b d** (tmux prefix then `d`); closing the browser tab also leaves the session running. If `portable agent` is already attached to a live session, extra arguments are ignored.

When the agent process exits, the tmux session drops to a login shell instead of restarting the agent on the next connect.

**Auth.** The Sprite URL is publicly reachable once remote is on, so the page is not an open shell. `portable remote` creates a random token, stores it only on the Sprite at `~/.config/portable/remote.token` (mode 600, not in your synced project), and puts it in the URL fragment (`#t=…`, not a query string). The page POSTs that token to `/login`, sets an HttpOnly cookie, and strips the fragment from the address bar. Wrong or missing tokens get a 401. Agent API keys stay in the Sprite home env files; they are not served over HTTP. Treat the URL like a password (screenshots). Rotate with `portable remote --reset-token`. Turn it off with `portable remote --off` (Sprite URL goes back to private / org-only). If making the URL private fails, that command errors instead of printing success.

**Sleep and sync.** A request to the URL wakes the Sprite and starts the web terminal service. An open tab keeps it awake. When you close the tab, the Sprite can sleep again (no extra paid service). Warm sleep keeps the tmux session; a **cold** wake starts a fresh one. Mutagen still runs on the laptop. If the laptop is asleep or you ran `portable down`, edits on the Sprite stay there until the laptop is back and sync resumes (`portable up` if you had paused it). Don't edit the same files on the laptop while it's disconnected if you want to avoid conflicts.

**Tests vs a real Sprite.** `go test` covers the web terminal (auth, CSP, PTY echo, disconnect killing the attach process), token/URL parsing, tmux wrap, install-script syntax, and release checksum verification. It does **not** call a live Fly Sprite, so `sprite url`, `sprite-env services create`, public URL, and wake-on-request are untested here.

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
