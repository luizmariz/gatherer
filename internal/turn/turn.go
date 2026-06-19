// Package turn is the reconcile engine. It groups a decklist's spells into the
// canonical phases of a Magic turn and resolves them in order.
package turn

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/luizmariz/gatherer/internal/deck"
	"github.com/luizmariz/gatherer/internal/ui"
)

// Step is one phase of a planned turn together with the spells cast during it.
type Step struct {
	Phase  Phase
	Spells []deck.Spell
}

// Turn is an ordered, validated reconcile plan for a single plane.
type Turn struct {
	Plane      string
	Permanents []deck.Permanent
	Steps      []Step
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

	t := &Turn{Plane: d.Plane, Permanents: d.Permanents}

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
// plan. Otherwise it casts each spell behind a spinner: a failed required spell
// is "countered" and aborts the turn, while a failed optional spell "fizzles".
func (t *Turn) Resolve(ctx context.Context, c Caster, opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	th := ui.For(out)

	verb := "Resolving"
	if opts.Scry {
		verb = "Scrying"
	}
	fmt.Fprintf(out, "\n%s %s the stack on plane %s\n", ui.Mana(), th.Title(verb), th.Title(quote(t.Plane)))
	if !opts.Scry {
		fmt.Fprintf(out, "%s\n", th.Flavor(ui.FlavorStart()))
	}

	if len(t.Permanents) > 0 {
		fmt.Fprintf(out, "\n%s %s — ensure prerequisite tech is in play\n", ui.IconReady, th.Title("Ready the battlefield"))

		for _, p := range t.Permanents {
			if err := ready(ctx, c, p, opts, out, th); err != nil {
				return err
			}
		}
	}

	for _, step := range t.Steps {
		fmt.Fprintf(out, "\n%s %s — %s\n", ui.PhaseIcon(step.Phase.Key()), th.Title(step.Phase.Title()), step.Phase.Intent())

		for _, s := range step.Spells {
			if opts.Scry {
				fmt.Fprintf(out, "  %s would cast %s: %s\n", ui.IconCast, quote(s.Name), th.Cmd(command(s)))
				continue
			}

			if err := castSpell(ctx, c, out, th, s); err != nil {
				return fmt.Errorf("%s countered in %s: %w", quote(s.Name), step.Phase.Title(), err)
			}
		}
	}

	if opts.Scry {
		fmt.Fprintf(out, "\n%s\n", th.Flavor("Scry complete — no spells were cast."))
	} else {
		fmt.Fprintf(out, "\n%s %s\n", th.OK(ui.IconResolve), th.OK("Turn passed — "+ui.FlavorWin()))
	}

	return nil
}

// castSpell runs one spell behind a spinner and prints a themed status line.
func castSpell(ctx context.Context, c Caster, out io.Writer, th ui.Theme, s deck.Spell) error {
	sp := th.Spinner(out, "casting "+quote(s.Name)+": "+th.Cmd(command(s)))
	start := time.Now()
	sp.Start()

	err := c.Cast(ctx, s)
	el := th.Cmd(" " + since(start))

	switch {
	case err == nil:
		sp.Stop(th.OK(ui.IconResolve+" "+quote(s.Name)+" resolves") + el)
		return nil
	case s.Optional:
		sp.Stop(th.Warn(ui.IconFizzle+" "+quote(s.Name)+" fizzles — "+ui.FlavorFizzle()) + el)
		return nil
	default:
		sp.Stop(th.Bad(ui.IconCounter+" "+quote(s.Name)+" countered — "+ui.FlavorCounter()) + el)
		return err
	}
}

// ready ensures one prerequisite permanent is in play: it pays the Cost, and if
// that is unmet, applies the permanent's Rules (or counters the turn when there
// are no Rules).
func ready(ctx context.Context, c Caster, p deck.Permanent, opts Options, out io.Writer, th ui.Theme) error {
	if opts.Scry {
		rules := "no rules (hard requirement)"
		if len(p.Rules) > 0 {
			rules = "rules: " + strings.Join(p.Rules, " ")
		}
		fmt.Fprintf(out, "  %s would pay cost for %s: %s  %s\n",
			ui.IconCast, quote(p.Name), th.Cmd(strings.Join(p.Cost, " ")), th.Flavor("(if unmet → "+rules+")"))
		return nil
	}

	sp := th.Spinner(out, "paying cost for "+quote(p.Name)+": "+th.Cmd(strings.Join(p.Cost, " ")))
	start := time.Now()
	sp.Start()
	costErr := c.Cast(ctx, deck.Spell{Name: p.Name + " (cost)", Cast: p.Cost})
	el := th.Cmd(" " + since(start))

	if costErr == nil {
		sp.Stop(th.OK(ui.IconInPlay+" "+quote(p.Name)+" already in play") + el)
		return nil
	}

	if len(p.Rules) == 0 {
		sp.Stop(th.Bad(ui.IconCounter+" "+quote(p.Name)+" not in play, no rules") + el)
		return fmt.Errorf("%s is not in play and has no rules — install it or add \"rules\"", quote(p.Name))
	}

	sp.Stop(th.Warn(ui.IconFizzle+" "+quote(p.Name)+" unmet — applying rules") + el)

	rsp := th.Spinner(out, "resolving "+quote(p.Name)+": "+th.Cmd(strings.Join(p.Rules, " ")))
	rstart := time.Now()
	rsp.Start()
	rulesErr := c.Cast(ctx, deck.Spell{Name: p.Name + " (rules)", Cast: p.Rules})
	rel := th.Cmd(" " + since(rstart))

	if rulesErr != nil {
		rsp.Stop(th.Bad(ui.IconCounter+" "+quote(p.Name)+" failed to resolve") + rel)
		return fmt.Errorf("failed to resolve %s into play: %w", quote(p.Name), rulesErr)
	}

	rsp.Stop(th.OK(ui.IconInPlay+" "+quote(p.Name)+" resolved into play") + rel)
	return nil
}

func command(s deck.Spell) string { return strings.Join(s.Cast, " ") }

func quote(s string) string { return "\"" + s + "\"" }

func since(t time.Time) string {
	d := time.Since(t)
	if d < time.Second {
		return fmt.Sprintf("(%dms)", d.Milliseconds())
	}
	return fmt.Sprintf("(%.1fs)", d.Seconds())
}
