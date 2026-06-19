// Package ui renders the turn with Magic-flavored, TTY-aware output: colors,
// icons, flavor text, and spinners. On a non-terminal (pipe/CI) it degrades to
// plain lines with no ANSI and no animation.
package ui

import (
	"io"
	"math/rand"
	"os"
)

// ANSI styles, emitted only when color is enabled.
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	italic  = "\033[3m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

// Icons (Unicode; render on non-color terminals too).
const (
	IconReady   = "◈"
	IconResolve = "✓"
	IconCounter = "✗"
	IconFizzle  = "≈"
	IconInPlay  = "◆"
	IconCast    = "•"
)

// phaseIcon maps a phase key to a glyph.
var phaseIcon = map[string]string{
	"untap":  "↺",
	"upkeep": "✦",
	"draw":   "✚",
	"main1":  "⚒",
	"combat": "⚔",
	"main2":  "✺",
	"end":    "☾",
}

// PhaseIcon returns the glyph for a phase key.
func PhaseIcon(key string) string {
	if g, ok := phaseIcon[key]; ok {
		return g
	}
	return IconCast
}

// mana are the five colors of Magic, used as a decorative flourish.
var mana = []string{"⚪", "🔵", "⚫", "🔴", "🟢"}

// Mana returns a random mana symbol.
func Mana() string { return mana[rand.Intn(len(mana))] }

var (
	flavorStart = []string{
		"Shuffle up and deal.",
		"Draw your opening hand — keep.",
		"On the play.",
		"The planeswalker steps onto a new plane.",
	}
	flavorWin = []string{
		"the stack resolves, priority passes.",
		"combat math checks out, the board is yours.",
		"the turn passes in good order.",
		"you untap, upkeep, and win.",
	}
	flavorCounter = []string{
		"the spell fizzles on the stack.",
		"Force of Will says no.",
		"exiled face-down.",
		"Misstep!",
	}
	flavorFizzle = []string{
		"no legal targets.",
		"trinket text; skipped.",
		"shrugged off.",
	}
)

func pick(s []string) string { return s[rand.Intn(len(s))] }

// FlavorStart, FlavorWin, FlavorCounter and FlavorFizzle return random quips.
func FlavorStart() string   { return pick(flavorStart) }
func FlavorWin() string     { return pick(flavorWin) }
func FlavorCounter() string { return pick(flavorCounter) }
func FlavorFizzle() string  { return pick(flavorFizzle) }

// Theme carries rendering capabilities derived from the output target.
type Theme struct {
	Color bool
	TTY   bool
}

// For returns a Theme for w: color when it's a terminal and NO_COLOR is unset.
func For(w io.Writer) Theme {
	tty := isTTY(w)
	return Theme{TTY: tty, Color: tty && os.Getenv("NO_COLOR") == ""}
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func (t Theme) paint(s, code string) string {
	if !t.Color {
		return s
	}
	return code + s + reset
}

// Style helpers.
func (t Theme) Title(s string) string  { return t.paint(s, bold+magenta) }
func (t Theme) Cmd(s string) string    { return t.paint(s, dim) }
func (t Theme) Flavor(s string) string { return t.paint(s, italic+cyan) }
func (t Theme) OK(s string) string     { return t.paint(s, green) }
func (t Theme) Bad(s string) string    { return t.paint(s, red) }
func (t Theme) Warn(s string) string   { return t.paint(s, yellow) }
