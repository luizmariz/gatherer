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
	"strings"
	"syscall"

	"github.com/luizmariz/gatherer/internal/deck"
	"github.com/luizmariz/gatherer/internal/remote"
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
	case "planeswalk":
		err = runPlaneswalk(args)
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
	verbose := fs.Bool("verbose", false, "stream raw command output live (disables the spinner)")
	from := fs.String("from", "", "resolve from this phase onward (untap|upkeep|draw|main1|combat|main2|end)")
	only := fs.String("only", "", "resolve only this phase")
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

	caster := &spell.Runner{Dir: *dir, Stream: *verbose}

	return plan.Resolve(ctx, caster, turn.Options{
		Scry:    scry,
		Verbose: *verbose,
		From:    *from,
		Only:    *only,
		Out:     os.Stdout,
	})
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

// runPlaneswalk ships the binary + working dir to a host over SSH and resolves
// the decklist there.
func runPlaneswalk(args []string) error {
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("usage: gatherer planeswalk <user@host> [--deck PATH] [--dir PATH]")
	}
	host := args[0]

	fs := flag.NewFlagSet("planeswalk", flag.ExitOnError)
	deckPath := fs.String("deck", "decklist.json", "decklist to resolve on the remote (must live inside --dir)")
	dir := fs.String("dir", ".", "local working directory to ship to the host")
	remoteDir := fs.String("remote-dir", "gatherer", "staging directory on the remote host")
	binary := fs.String("binary", "", "gatherer binary to ship (default: this executable)")
	scry := fs.Bool("scry", false, "dry-run on the remote instead of resolving")
	fs.Parse(args[1:])

	bin := *binary
	if bin == "" {
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locating this binary to ship: %w", err)
		}
		bin = self
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return remote.Planeswalk(ctx, remote.Options{
		Host:       host,
		RemoteDir:  *remoteDir,
		LocalDir:   *dir,
		DeckPath:   *deckPath,
		BinaryPath: bin,
		Scry:       *scry,
		Out:        os.Stdout,
	})
}

func usage() {
	fmt.Print(`gatherer — reconcile a host by playing a Magic turn

Usage:
  gatherer <command> [flags]

Commands:
  resolve      Resolve the stack: run the full reconcile turn
  scry         Dry-run: announce every spell without casting it
  oracle       Show the canonical desired state in turn order
  planeswalk   Ship gatherer + a decklist to a host over SSH and resolve it there
  version      Print the version
  help         Show this help

Common flags:
  --deck PATH  Decklist file (default "decklist.json")
  --dir  PATH  Working directory for casts (default ".")

planeswalk:
  gatherer planeswalk <user@host> [--deck PATH] [--dir PATH] [--remote-dir PATH]
                       [--binary PATH] [--scry]

A turn runs these phases in order:
  untap → upkeep → draw → main1 → combat → main2 → end
`)
}
