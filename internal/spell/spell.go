// Package spell casts spells for real: it executes a decklist command as a
// child process, capturing output so the caller can render clean status lines
// and surface logs only when a cast fails.
package spell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/luizmariz/gatherer/internal/deck"
)

// Runner casts spells by running their command on the host.
type Runner struct {
	Dir    string   // working directory for casts (usually the repo root)
	Env    []string // extra environment, appended to the host environment
	Stream bool     // also stream output live to the terminal (verbose mode)
}

// Cast runs the spell's command and waits for it to finish. Output is captured;
// on failure it is attached (indented) to the returned error for debugging.
func (r *Runner) Cast(ctx context.Context, s deck.Spell) error {
	if len(s.Cast) == 0 {
		return fmt.Errorf("spell %q has no command", s.Name)
	}

	cmd := exec.CommandContext(ctx, s.Cast[0], s.Cast[1:]...)
	cmd.Dir = r.Dir

	var buf bytes.Buffer
	if r.Stream {
		cmd.Stdout = io.MultiWriter(&buf, os.Stdout)
		cmd.Stderr = io.MultiWriter(&buf, os.Stderr)
	} else {
		cmd.Stdout = &buf
		cmd.Stderr = &buf
	}

	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
	}

	if err := cmd.Run(); err != nil {
		// In stream mode the output already reached the terminal live, so don't
		// repeat it in the error.
		if !r.Stream {
			if out := strings.TrimSpace(buf.String()); out != "" {
				return fmt.Errorf("%s: %w\n%s", s.Cast[0], err, indent(out))
			}
		}
		return fmt.Errorf("%s: %w", s.Cast[0], err)
	}

	return nil
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "      │ " + l
	}
	return strings.Join(lines, "\n")
}
