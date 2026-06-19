// Package turn is the reconcile engine. It groups a decklist's spells into the
// canonical phases of a Magic turn and resolves them in order.
package turn

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/luizmariz/gatherer/internal/deck"
)

// Step is one phase of a planned turn together with the spells cast during it.
type Step struct {
	Phase  Phase
	Spells []deck.Spell
}

// Turn is an ordered, validated reconcile plan for a single plane.
type Turn struct {
	Plane string
	Steps []Step
}

// Caster executes a single spell. spell.Runner is the real implementation;
// tests supply a fake. The engine depends only on this interface.
type Caster interface {
	Cast(ctx context.Context, s deck.Spell) error
}

// Options tune a resolve.
type Options struct {
	Scry bool      // plan only: announce spells without casting them
	Out  io.Writer // where banners and flavor are written
}

// Plan groups the decklist's spells by phase and orders them as a turn.
func Plan(d *deck.Decklist) (*Turn, error) {
	grouped := map[Phase][]deck.Spell{}

	for _, s := range d.Spells {
		p, err := ParsePhase(s.Phase)
		if err != nil {
			return nil, fmt.Errorf("spell %q: %w", s.Name, err)
		}
		grouped[p] = append(grouped[p], s)
	}

	t := &Turn{Plane: d.Plane}

	for _, p := range Phases() {
		spells := grouped[p]
		if len(spells) == 0 {
			continue
		}
		t.Steps = append(t.Steps, Step{Phase: p, Spells: spells})
	}

	return t, nil
}

// Resolve walks the turn phase by phase. In scry mode it only announces the
// plan. Otherwise it casts each spell: a failed required spell is "countered"
// and aborts the turn, while a failed optional spell "fizzles" and is skipped.
func (t *Turn) Resolve(ctx context.Context, c Caster, opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	verb := "Resolving"
	if opts.Scry {
		verb = "Scrying"
	}
	fmt.Fprintf(out, "\n%s the stack on plane %q\n", verb, t.Plane)

	for _, step := range t.Steps {
		fmt.Fprintf(out, "\n— %s — %s\n", step.Phase.Title(), step.Phase.Intent())

		for _, s := range step.Spells {
			if opts.Scry {
				fmt.Fprintf(out, "  · would cast %q: %s\n", s.Name, command(s))
				continue
			}

			fmt.Fprintf(out, "  · casting %q: %s\n", s.Name, command(s))

			if err := c.Cast(ctx, s); err != nil {
				if s.Optional {
					fmt.Fprintf(out, "    ~ %q fizzles (optional): %v\n", s.Name, err)
					continue
				}
				return fmt.Errorf("%q countered in %s: %w", s.Name, step.Phase.Title(), err)
			}

			fmt.Fprintf(out, "    ✓ %q resolves\n", s.Name)
		}
	}

	if opts.Scry {
		fmt.Fprintf(out, "\nScry complete — no spells were cast.\n")
	} else {
		fmt.Fprintf(out, "\nTurn passed — the battlefield matches the decklist.\n")
	}

	return nil
}

func command(s deck.Spell) string {
	return strings.Join(s.Cast, " ")
}
