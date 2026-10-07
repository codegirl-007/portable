# portable

Use a fast, persistent Fly workspace for a project while keeping the laptop's
project directory as the source of truth. `portable` syncs files both ways with
Mutagen, then runs tools such as OpenCode and Neovim in the remote workspace.

## Install

Requirements: Go, the [Sprite CLI](https://fly.io/sprites/) (authenticated with
`sprite login`), and [Mutagen](https://github.com/mutagen-io/mutagen/releases).
Install both Mutagen release artifacts: `mutagen` on `PATH` and
`mutagen-agents.tar.gz` in `~/.local/libexec/`.

Build and install portable from this repository:

```sh
go build -o ~/.local/bin/portable .
```

## Use

Run commands from the project directory:

```sh
cd ~/projects/my-project
portable up                 # create/wake workspace, sync files, install deps
portable opencode           # OpenCode TUI in the remote project
portable nvim               # Neovim in the remote project
portable run go test ./...  # Run a command remotely
portable ssh                # Open a remote shell
portable status
portable down               # Pause sync; workspace sleeps
portable destroy            # Permanently delete workspace and its data
```

After the first setup, `up` normally resumes in a couple of seconds. Changes
made locally or remotely sync automatically. `portable opencode --continue`
continues the last OpenCode session.

## Configuration

There are two separate config files:

- **Global defaults**, on your laptop: `~/.config/portable/config.toml`.
- **Project-specific settings**, inside that project's directory:
  `~/projects/my-project/.portable.yaml`.

For example, set shared defaults globally:

```toml
# ~/.config/portable/config.toml
sprite_prefix = "portable"
sync_ignores = [".git", "node_modules", "target", "dist", "build"]
dotfiles = [".config/nvim", ".tmux.conf", ".gitconfig"]
tools = ["GOBIN=\"$HOME/.local/bin\" go install example.com/tool/cmd/tool@latest"]
```

```yaml
# ~/projects/my-project/.portable.yaml
package_manager: bun # optional; normally detected from the project files
```

`dotfiles` lists home-relative paths copied once when the workspace is
provisioned; use `portable up --reprovision` to copy them again and rerun
one-time provisioning.
`tools` are one-time provisioning commands. Portable detects the package
manager from files such as `package.json` and its lockfile, then runs the usual
install command (`bun install`, `npm ci`, etc.). You normally don't need to
configure `package_manager`; set it only if detection picks the wrong manager.

OpenCode Go credentials are read from the local OpenCode setup and provisioned
to the remote workspace. Other secrets copied in dotfiles or provisioning
commands are stored on that workspace—avoid putting credentials in synced
project files.
