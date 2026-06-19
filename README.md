# gatherer

A personal ops CLI that reconciles a host to a desired state by **playing a Magic: The Gathering turn**.

A reconcile *is* a turn. You declare the desired state as a **decklist** (JSON);
`gatherer` groups your steps ("spells") into the canonical phases of a Magic turn
and resolves them in order. The engine is generic — it knows nothing about any
specific project. ServiPago is just one decklist.

Single static Go binary, zero dependencies: `go build`, `scp` it to any Linux
host, run. That's the whole point — deploy anywhere, no runtime, no lock-in.

## The mapping

A turn runs these phases in order. Each maps to a stage of a reconcile:

| Phase   | MTG step           | Reconcile intent                                   |
| ------- | ------------------ | -------------------------------------------------- |
| `untap` | Untap Step         | Ready the host: Docker and Swarm online            |
| `upkeep`| Upkeep Step        | Pay costs: verify secrets and prerequisites        |
| `draw`  | Draw Step          | Draw resources: pull images and source             |
| `main1` | First Main Phase   | Develop the board: deploy platform services        |
| `combat`| Combat Phase       | Declare attackers: bring application services live |
| `main2` | Second Main Phase  | Post-combat: bootstrap and migrate                 |
| `end`   | End Step           | Cleanup: health checks and pruning                 |

Other vocabulary:

- **Decklist** — your declarative desired state (`decklist.json`).
- **Spell** — one reconcile step: a command bound to a phase. Failure "counters"
  the turn; an `optional` spell that fails "fizzles" and is skipped.
- **Permanent** — a prerequisite ("tech") that must be in play before the turn
  casts spells. Its `cost` tests whether it's present; if not, its `rules` install
  it. A permanent whose `cost` is unmet and that has no `rules` counters the turn.
- **Scry** — look before you leap: a dry-run that announces every spell without
  casting it.
- **Oracle** — the canonical, authoritative desired state, printed in turn order.

## Usage

```sh
make build                                  # -> ./gatherer

./gatherer scry --deck decklist.example.json     # dry-run the turn
./gatherer oracle --deck decklist.example.json   # canonical desired state, in order
./gatherer resolve --deck decklist.json --dir /opt/platform   # do it for real
./gatherer planeswalk deploy@vps --deck decklist.example.json # set up a remote host from scratch
```

Commands:

| Command       | What it does                                         |
| ------------- | ---------------------------------------------------- |
| `resolve`     | Resolve the stack: run the full reconcile turn       |
| `scry`        | Dry-run: announce every spell without casting it     |
| `oracle`      | Show the canonical desired state in turn order       |
| `planeswalk`  | Ship gatherer + a decklist to a host over SSH and resolve it there |
| `version`     | Print the version                                    |

Common flags:

- `--deck PATH` — decklist file (default `decklist.json`)
- `--dir PATH` — working directory for casts (default `.`)
- `--verbose` — stream raw command output live (disables the spinner)
- `--from PHASE` / `--only PHASE` — resolve a slice of the turn (e.g. re-run from
  `combat`, or just the `end` health checks). The `permanents` preflight always runs.

## Decklist format

```json
{
  "plane": "production",
  "permanents": [
    {
      "name": "docker",
      "cost":  ["sh", "-c", "command -v docker >/dev/null"],
      "rules": ["sh", "-c", "apk add --no-cache docker docker-cli-compose && rc-update add docker default && service docker start"]
    }
  ],
  "spells": [
    { "name": "ready-docker",  "phase": "untap",  "cast": ["docker", "info"] },
    { "name": "deploy-app",    "phase": "combat", "cast": ["docker", "stack", "deploy", "-c", "docker-compose.yml", "servipago"] },
    { "name": "bootstrap",     "phase": "main2",  "optional": true, "cast": ["sh", "garage/bootstrap.sh"] }
  ]
}
```

- `permanents` run first as a preflight: each `cost` is tested; if it fails and
  `rules` exist, the permanent is installed; if it fails with no `rules`, the turn
  is countered. `rules` is optional — omit it for a hard "must already be present".
- `cast`, `cost` and `rules` are argv arrays (no shell parsing — wrap in `sh -c`
  for shell features). Output is captured and surfaced only when a step fails.
- Spells may be listed in any order; the engine reorders them into turn order by phase.
- Unknown JSON fields are rejected, so typos surface immediately.

See `decklist.example.json` for a full ServiPago-shaped reconcile.

## Project layout

```
cmd/gatherer/      CLI entrypoint and command dispatch
internal/deck/     decklist data model + JSON loader/validator
internal/turn/     the reconcile engine: phases + planning + resolve
internal/spell/    the caster: executes a spell as a child process
internal/ui/       TTY-aware rendering: colors, MTG icons, flavor, spinners
internal/remote/   planeswalk: ship + resolve on a host over ssh/rsync/scp
```

## Planeswalk (remote)

`planeswalk` sets up a host from scratch — no runtime to install, because gatherer
is a single static binary:

```sh
gatherer planeswalk deploy@vps --deck decklist.production.json --dir .
```

It (1) scouts the plane (`uname` to detect OS/arch), (2) stages a working dir on
the remote, (3) `rsync`s your `--dir` (decklist + compose + scripts) there,
(4) `scp`s the binary, then (5) runs `gatherer resolve` **on the host** over an
interactive SSH session, so its spinners and colors render locally. On a bare box
the `permanents` install the missing tech (e.g. docker) before the spells deploy.

- Uses the system `ssh`/`rsync`/`scp` — your SSH config, keys, and Tailscale names
  all just work. Falls back to a tar-over-ssh pipeline when `rsync` is absent.
- The decklist must live inside `--dir`. Add `--scry` to dry-run on the remote,
  `--binary PATH` to ship a cross-compiled build for a different remote arch.

Dependency direction is one-way: `turn` consumes `deck`; `spell` consumes `deck`;
`main` wires them together. `turn` depends only on a `Caster` interface, so the
engine is fully unit-tested with a fake caster (no Docker needed).

## Develop

```sh
make test    # unit tests (engine + loader)
make vet     # go vet
make scry    # build + dry-run against the example decklist
```

## Roadmap

- `counterspell`: abort an in-flight resolve cleanly (context cancellation is wired).
- `planeswalk --secrets`: securely seed host secrets (ties into the secret-tools store).
