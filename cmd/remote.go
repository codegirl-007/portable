package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/ui"
)

// upStep prints a step line only in verbose mode, so `up` stays quiet.
func upStep(format string, args ...any) {
	if flagVerbose {
		ui.Step(format, args...)
	}
}

// remoteRun runs a command on the Sprite. Unless --verbose is set, output is
// captured and only printed if the command fails.
func remoteRun(client *sprites.Client, name string, opts sprites.ExecOptions, command ...string) error {
	if flagVerbose {
		return client.Exec(bg(), name, opts, command...)
	}
	var buf bytes.Buffer
	opts.Stdout = &buf
	opts.Stderr = &buf
	if err := client.Exec(bg(), name, opts, command...); err != nil {
		fmt.Fprint(os.Stderr, buf.String())
		return err
	}
	return nil
}
