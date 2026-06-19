// Package spell casts spells for real: it executes a decklist command as a
// child process, streaming its output to the host.
package spell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/luizmariz/gatherer/internal/deck"
)

// Runner casts spells by running their command on the host.
type Runner struct {
	Dir string    // working directory for casts (usually the repo root)
	Env []string  // extra environment, appended to the host environment
	Out io.Writer // child stdout (defaults to os.Stdout)
	Err io.Writer // child stderr (defaults to os.Stderr)
}

// Cast runs the spell's command and waits for it to finish.
func (r *Runner) Cast(ctx context.Context, s deck.Spell) error {
	if len(s.Cast) == 0 {
		return fmt.Errorf("spell %q has no command", s.Name)
	}

	cmd := exec.CommandContext(ctx, s.Cast[0], s.Cast[1:]...)
	cmd.Dir = r.Dir
	cmd.Stdout = orDefault(r.Out, os.Stdout)
	cmd.Stderr = orDefault(r.Err, os.Stderr)

	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", s.Cast[0], err)
	}

	return nil
}

func orDefault(w, def io.Writer) io.Writer {
	if w == nil {
		return def
	}
	return w
}
