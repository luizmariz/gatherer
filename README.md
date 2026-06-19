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
- **Permanent** — a service you expect on the battlefield (flavor: land, artifact,
  creature, ...). Listed by `battlefield`.
- **Scry** — look before you leap: a dry-run that announces every spell without
  casting it.
- **Oracle** — the canonical, authoritative desired state, printed in turn order.

## Usage

```sh
make build                                  # -> ./gatherer

./gatherer scry --deck decklist.example.json     # dry-run the turn
./gatherer oracle --deck decklist.example.json   # canonical desired state, in order
./gatherer battlefield --deck decklist.example.json
./gatherer resolve --deck decklist.json --dir /opt/platform   # do it for real
```

Commands:

| Command       | What it does                                         |
| ------------- | ---------------------------------------------------- |
| `resolve`     | Resolve the stack: run the full reconcile turn       |
| `scry`        | Dry-run: announce every spell without casting it     |
| `battlefield` | Show the permanents (services) declared for the plane|
| `oracle`      | Show the canonical desired state in turn order       |
| `version`     | Print the version                                    |

Common flags: `--deck PATH` (default `decklist.json`), `--dir PATH` (working
directory for casts, default `.`).

## Decklist format

```json
{
  "plane": "production",
  "permanents": [{ "name": "postgres", "type": "land" }],
  "spells": [
    { "name": "ready-docker",  "phase": "untap",  "cast": ["docker", "info"] },
    { "name": "deploy-app",    "phase": "combat", "cast": ["docker", "stack", "deploy", "-c", "docker-compose.yml", "servipago"] },
    { "name": "bootstrap",     "phase": "main2",  "optional": true, "cast": ["sh", "garage/bootstrap.sh"] }
  ]
}
```

- `cast` is an argv array (no shell parsing), run as a child process with stdout/stderr streamed.
- Spells may be listed in any order; the engine reorders them into turn order by phase.
- Unknown JSON fields are rejected, so typos surface immediately.

See `decklist.example.json` for a full ServiPago-shaped reconcile.

## Project layout

```
cmd/gatherer/      CLI entrypoint and command dispatch
internal/deck/     decklist data model + JSON loader/validator
internal/turn/     the reconcile engine: phases + planning + resolve
internal/spell/    the caster: executes a spell as a child process
```

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

- `--only <phase>` / `--from <phase>` to resolve a slice of the turn.
- `counterspell`: abort an in-flight resolve cleanly (context cancellation is wired).
- Remote casting over SSH/Tailscale so CI can `resolve` a host directly.
- `battlefield` reading live `docker stack services` instead of only the decklist.
