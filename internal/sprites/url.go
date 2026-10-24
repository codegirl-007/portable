package sprites

import (
	"context"
	"fmt"
	"strings"

	"github.com/codegirl-007/portable/internal/remote"
)

// HTTPURL returns the Sprite HTTPS URL and url-auth mode.
func (c *Client) HTTPURL(ctx context.Context, name string) (remote.SpriteHTTP, error) {
	out, err := c.run(ctx, "api", "/v1/sprites/"+name)
	if err == nil {
		info := remote.ParseSpriteHTTP(out)
		if info.URL != "" {
			return info, nil
		}
	}
	out, err = c.run(ctx, "url", "-s", name)
	if err != nil {
		out, err = c.run(ctx, "info", "-s", name)
		if err != nil {
			return remote.SpriteHTTP{}, err
		}
	}
	info := remote.ParseSpriteHTTP(out)
	if info.URL == "" {
		return remote.SpriteHTTP{}, fmt.Errorf("could not find a sprites.app URL in sprite CLI output")
	}
	return info, nil
}

// SetURLAuth sets Sprite URL authentication to "public" or "sprite".
func (c *Client) SetURLAuth(ctx context.Context, name, auth string) error {
	auth = strings.TrimSpace(auth)
	if _, err := c.run(ctx, "url", "update", "--auth", auth, "-s", name); err == nil {
		return nil
	}
	_, err := c.run(ctx, "config", "update", "--url-auth", auth, "-s", name)
	return err
}
