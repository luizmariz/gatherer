// Command gatherer is a personal ops CLI that reconciles a host to a desired
// state by playing a Magic: The Gathering "turn". See README for the mapping.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/luizmariz/gatherer/internal/deck"
	"github.com/luizmariz/gatherer/internal/spell"
	"github.com/luizmariz/gatherer/internal/turn"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	args := os.Args[2:]

	var err error
	switch os.Args[1] {
	case "resolve":
		err = runTurn(args, false)
	case "scry":
		err = runTurn(args, true)
	case "oracle":
		err = runOracle(args)
	case "version", "--version", "-v":
		fmt.Printf("gatherer %s — \"Knowledge is the difference between a deck and a pile of cards.\"\n", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n✗ %v\n", err)
		os.Exit(1)
	}
}

// runTurn resolves (or scries) the full reconcile turn from a decklist.
func runTurn(args []string, scry bool) error {
	name := "resolve"
	if scry {
		name = "scry"
	}

	fs := flag.NewFlagSet(name, flag.ExitOnError)
	deckPath := fs.String("deck", "decklist.json", "path to the decklist (desired state)")
	dir := fs.String("dir", ".", "working directory for casting spells")
	fs.Parse(args)

	d, err := deck.Load(*deckPath)
	if err != nil {
		return err
	}

	plan, err := turn.Plan(d)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	caster := &spell.Runner{Dir: *dir}

	return plan.Resolve(ctx, caster, turn.Options{Scry: scry, Out: os.Stdout})
}

// runOracle prints the canonical desired state in turn order.
func runOracle(args []string) error {
	fs := flag.NewFlagSet("oracle", flag.ExitOnError)
	deckPath := fs.String("deck", "decklist.json", "path to the decklist")
	asJSON := fs.Bool("json", false, "print the parsed decklist as JSON")
	fs.Parse(args)

	d, err := deck.Load(*deckPath)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	plan, err := turn.Plan(d)
	if err != nil {
		return err
	}

	fmt.Printf("Oracle text for plane %q (canonical order):\n", plan.Plane)

	for _, step := range plan.Steps {
		fmt.Printf("\n%s — %s\n", step.Phase.Title(), step.Phase.Intent())

		for _, s := range step.Spells {
			tag := ""
			if s.Optional {
				tag = " (optional)"
			}
			fmt.Printf("  %s%s\n", s.Name, tag)
		}
	}

	return nil
}

func usage() {
	fmt.Print(`gatherer — reconcile a host by playing a Magic turn

Usage:
  gatherer <command> [flags]

Commands:
  resolve      Resolve the stack: run the full reconcile turn
  scry         Dry-run: announce every spell without casting it
  oracle       Show the canonical desired state in turn order
  version      Print the version
  help         Show this help

Common flags:
  --deck PATH  Decklist file (default "decklist.json")
  --dir  PATH  Working directory for casts (default ".")

A turn runs these phases in order:
  untap → upkeep → draw → main1 → combat → main2 → end
`)
}
