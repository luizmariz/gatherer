package turn

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/luizmariz/gatherer/internal/deck"
)

type fakeCaster struct {
	cast []string
	fail map[string]error
}

func (f *fakeCaster) Cast(_ context.Context, s deck.Spell) error {
	f.cast = append(f.cast, s.Name)
	return f.fail[s.Name]
}

// sampleDeck intentionally lists spells out of phase order to prove the engine
// reorders them into canonical turn order.
func sampleDeck() *deck.Decklist {
	return &deck.Decklist{
		Plane: "test",
		Spells: []deck.Spell{
			{Name: "go-live", Phase: "combat", Cast: []string{"true"}},
			{Name: "ready", Phase: "untap", Cast: []string{"true"}},
			{Name: "fetch", Phase: "draw", Cast: []string{"true"}},
		},
	}
}

func TestPlanOrdersByPhase(t *testing.T) {
	plan, err := Plan(sampleDeck())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	want := []Phase{Untap, Draw, Combat}

	if len(plan.Steps) != len(want) {
		t.Fatalf("got %d steps, want %d", len(plan.Steps), len(want))
	}
	for i, p := range want {
		if plan.Steps[i].Phase != p {
			t.Errorf("step %d: got %s, want %s", i, plan.Steps[i].Phase, p)
		}
	}
}

func TestPlanRejectsUnknownPhase(t *testing.T) {
	d := &deck.Decklist{
		Plane:  "test",
		Spells: []deck.Spell{{Name: "x", Phase: "morph", Cast: []string{"true"}}},
	}

	if _, err := Plan(d); err == nil {
		t.Fatal("expected error for unknown phase")
	}
}

func TestScryCastsNothing(t *testing.T) {
	plan, _ := Plan(sampleDeck())
	c := &fakeCaster{}

	if err := plan.Resolve(context.Background(), c, Options{Scry: true, Out: io.Discard}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(c.cast) != 0 {
		t.Fatalf("scry cast %d spells, want 0", len(c.cast))
	}
}

func TestResolveCastsInPhaseOrder(t *testing.T) {
	plan, _ := Plan(sampleDeck())
	c := &fakeCaster{}

	if err := plan.Resolve(context.Background(), c, Options{Out: io.Discard}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := []string{"ready", "fetch", "go-live"}

	if len(c.cast) != len(want) {
		t.Fatalf("cast %v, want %v", c.cast, want)
	}
	for i := range want {
		if c.cast[i] != want[i] {
			t.Errorf("cast[%d]=%s, want %s", i, c.cast[i], want[i])
		}
	}
}

func TestRequiredFailureAbortsTurn(t *testing.T) {
	plan, _ := Plan(sampleDeck())
	c := &fakeCaster{fail: map[string]error{"fetch": errors.New("boom")}}

	err := plan.Resolve(context.Background(), c, Options{Out: io.Discard})
	if err == nil {
		t.Fatal("expected the turn to be countered")
	}

	// go-live is in combat, after the countered draw spell, so it must not run.
	for _, name := range c.cast {
		if name == "go-live" {
			t.Fatal("turn continued past a countered spell")
		}
	}
}

func TestResolveOnlyPhase(t *testing.T) {
	plan, _ := Plan(sampleDeck()) // untap:ready, draw:fetch, combat:go-live
	c := &fakeCaster{}

	if err := plan.Resolve(context.Background(), c, Options{Only: "draw", Out: io.Discard}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(c.cast) != 1 || c.cast[0] != "fetch" {
		t.Fatalf("--only draw cast %v, want [fetch]", c.cast)
	}
}

func TestResolveFromPhase(t *testing.T) {
	plan, _ := Plan(sampleDeck())
	c := &fakeCaster{}

	if err := plan.Resolve(context.Background(), c, Options{From: "draw", Out: io.Discard}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := []string{"fetch", "go-live"} // draw onward, skipping untap's "ready"
	if len(c.cast) != len(want) {
		t.Fatalf("--from draw cast %v, want %v", c.cast, want)
	}
	for i := range want {
		if c.cast[i] != want[i] {
			t.Errorf("cast[%d]=%s, want %s", i, c.cast[i], want[i])
		}
	}
}

func TestResolveRejectsBadPhaseFilter(t *testing.T) {
	plan, _ := Plan(sampleDeck())

	if err := plan.Resolve(context.Background(), &fakeCaster{}, Options{Only: "morph", Out: io.Discard}); err == nil {
		t.Fatal("expected error for unknown phase in --only")
	}
}

func TestPermanentPresentIsBypassed(t *testing.T) {
	d := &deck.Decklist{
		Plane:      "test",
		Permanents: []deck.Permanent{{Name: "docker", Cost: []string{"true"}, Rules: []string{"install"}}},
		Spells:     []deck.Spell{{Name: "go", Phase: "untap", Cast: []string{"true"}}},
	}
	plan, _ := Plan(d)
	c := &fakeCaster{} // "docker (cost)" succeeds (no fail entry)

	if err := plan.Resolve(context.Background(), c, Options{Out: io.Discard}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	for _, name := range c.cast {
		if name == "docker (rules)" {
			t.Fatal("applied rules to a permanent that was already in play")
		}
	}
}

func TestPermanentUnmetCostAppliesRules(t *testing.T) {
	d := &deck.Decklist{
		Plane:      "test",
		Permanents: []deck.Permanent{{Name: "docker", Cost: []string{"false"}, Rules: []string{"install"}}},
		Spells:     []deck.Spell{{Name: "go", Phase: "untap", Cast: []string{"true"}}},
	}
	plan, _ := Plan(d)
	c := &fakeCaster{fail: map[string]error{"docker (cost)": errors.New("unmet")}}

	if err := plan.Resolve(context.Background(), c, Options{Out: io.Discard}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	applied := false
	for _, name := range c.cast {
		if name == "docker (rules)" {
			applied = true
		}
	}
	if !applied {
		t.Fatal("rules were not applied for an unmet permanent")
	}
}

func TestPermanentUnmetWithoutRulesCountersTurn(t *testing.T) {
	d := &deck.Decklist{
		Plane:      "test",
		Permanents: []deck.Permanent{{Name: "docker", Cost: []string{"false"}}},
		Spells:     []deck.Spell{{Name: "go", Phase: "untap", Cast: []string{"true"}}},
	}
	plan, _ := Plan(d)
	c := &fakeCaster{fail: map[string]error{"docker (cost)": errors.New("unmet")}}

	err := plan.Resolve(context.Background(), c, Options{Out: io.Discard})
	if err == nil {
		t.Fatal("expected the turn to be countered when a required permanent's cost is unmet")
	}

	for _, name := range c.cast {
		if name == "go" {
			t.Fatal("turn continued past a missing required permanent")
		}
	}
}

func TestOptionalFailureContinues(t *testing.T) {
	d := sampleDeck()
	d.Spells[2].Optional = true // fetch (draw) is optional

	plan, _ := Plan(d)
	c := &fakeCaster{fail: map[string]error{"fetch": errors.New("boom")}}

	if err := plan.Resolve(context.Background(), c, Options{Out: io.Discard}); err != nil {
		t.Fatalf("optional failure should not abort: %v", err)
	}

	found := false
	for _, name := range c.cast {
		if name == "go-live" {
			found = true
		}
	}
	if !found {
		t.Fatal("turn did not continue after an optional fizzle")
	}
}
