package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// DotfileOption is a suggested home-relative path with a short explanation.
type DotfileOption struct {
	Path string
	Hint string
}

// DotfileSuggestions returns common dotfile paths to offer during setup.
func DotfileSuggestions(nvimOnWorkspace bool) []DotfileOption {
	opts := []DotfileOption{
		{Path: ".gitconfig", Hint: "git identity and aliases"},
	}
	if nvimOnWorkspace {
		opts = append(opts, DotfileOption{
			Path: ".config/nvim",
			Hint: "Neovim config (Lazy.nvim sync runs on first up when this is present)",
		})
	} else {
		opts = append(opts, DotfileOption{
			Path: ".config/nvim",
			Hint: "Neovim config (only useful if you install Neovim on the workspace)",
		})
	}
	opts = append(opts,
		DotfileOption{Path: ".tmux.conf", Hint: "tmux settings for portable ssh"},
		DotfileOption{Path: ".config/starship.toml", Hint: "Starship prompt (optional)"},
	)
	return opts
}

// FilterExisting returns paths from candidates that exist under home.
func FilterExisting(home string, candidates []string) []string {
	var out []string
	for _, rel := range candidates {
		if homePathExists(home, rel) {
			out = append(out, rel)
		}
	}
	return out
}

// CandidatePaths returns just the path strings from options.
func CandidatePaths(opts []DotfileOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Path
	}
	return out
}

func homePathExists(home, rel string) bool {
	rel = strings.TrimPrefix(strings.TrimSpace(rel), "~/")
	if rel == "" {
		return false
	}
	p := filepath.Join(home, rel)
	if st, err := os.Stat(p); err == nil {
		return st.Mode().IsRegular() || st.IsDir()
	}
	// Follow one level of symlink for common dotfile links.
	if st, err := os.Lstat(p); err == nil && st.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Stat(p); err == nil {
			return target.Mode().IsRegular() || target.IsDir()
		}
	}
	return false
}
