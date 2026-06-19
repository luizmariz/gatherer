// Package deck loads the declarative desired state — a "decklist" — that a
// turn resolves. A decklist is plain JSON so the binary stays dependency-free.
package deck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Decklist is the desired state for one plane (environment / host).
type Decklist struct {
	Plane      string      `json:"plane"`
	Permanents []Permanent `json:"permanents,omitempty"`
	Spells     []Spell     `json:"spells"`
}

// Permanent is a service expected to be on the battlefield. Type is flavor
// only (land, artifact, creature, ...) and does not affect reconciliation.
type Permanent struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// Spell is a single reconcile step: a command (Cast) bound to a turn phase.
// An optional spell that fails fizzles and is skipped instead of aborting.
type Spell struct {
	Name     string   `json:"name"`
	Phase    string   `json:"phase"`
	Cast     []string `json:"cast"`
	Optional bool     `json:"optional,omitempty"`
}

// Load reads and structurally validates a decklist from a JSON file.
func Load(path string) (*Decklist, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read decklist: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	var d Decklist
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("parse decklist %s: %w", path, err)
	}

	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("invalid decklist %s: %w", path, err)
	}

	return &d, nil
}

// Validate checks structural integrity. Phase keys are validated later, when
// the turn is planned, so deck stays independent of the turn engine.
func (d *Decklist) Validate() error {
	if d.Plane == "" {
		return errors.New("plane must be set")
	}

	if len(d.Spells) == 0 {
		return errors.New("decklist has no spells to cast")
	}

	for i, s := range d.Spells {
		switch {
		case s.Name == "":
			return fmt.Errorf("spell %d: name must be set", i)
		case s.Phase == "":
			return fmt.Errorf("spell %q: phase must be set", s.Name)
		case len(s.Cast) == 0:
			return fmt.Errorf("spell %q: cast must list a command to run", s.Name)
		}
	}

	return nil
}
