package turn

import (
	"fmt"
	"strings"
)

// Phase is one step of a Magic turn, reused here as a stage of a reconcile.
// The zero value is Untap; Phases() returns every phase in canonical order.
type Phase int

const (
	Untap Phase = iota
	Upkeep
	Draw
	Main1
	Combat
	Main2
	End
)

type phaseInfo struct {
	key    string
	title  string
	intent string
}

var phases = map[Phase]phaseInfo{
	Untap:  {"untap", "Untap Step", "ready the host: Docker and Swarm online"},
	Upkeep: {"upkeep", "Upkeep Step", "pay costs: verify secrets and prerequisites"},
	Draw:   {"draw", "Draw Step", "draw resources: pull images and source"},
	Main1:  {"main1", "First Main Phase", "develop the board: deploy platform services"},
	Combat: {"combat", "Combat Phase", "declare attackers: bring application services live"},
	Main2:  {"main2", "Second Main Phase", "post-combat: bootstrap and migrate"},
	End:    {"end", "End Step", "cleanup: health checks and pruning"},
}

var order = []Phase{Untap, Upkeep, Draw, Main1, Combat, Main2, End}

// Phases returns every phase in canonical turn order.
func Phases() []Phase {
	out := make([]Phase, len(order))
	copy(out, order)
	return out
}

func (p Phase) Key() string    { return phases[p].key }
func (p Phase) Title() string  { return phases[p].title }
func (p Phase) Intent() string { return phases[p].intent }
func (p Phase) String() string { return phases[p].key }

// ParsePhase resolves a decklist phase key (case-insensitive) to a Phase.
func ParsePhase(s string) (Phase, error) {
	want := strings.ToLower(strings.TrimSpace(s))

	for _, p := range order {
		if phases[p].key == want {
			return p, nil
		}
	}

	return 0, fmt.Errorf("unknown phase %q (valid: %s)", s, strings.Join(keys(), ", "))
}

func keys() []string {
	out := make([]string, len(order))
	for i, p := range order {
		out[i] = phases[p].key
	}
	return out
}
