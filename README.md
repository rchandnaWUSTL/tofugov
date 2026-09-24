# tofugov

Bulk provider upgrades for OpenTofu, with a plain-English explanation and a risk score for every plan.

![tofugov demo](demo/video/out/tofugov-demo.gif)

Bumping a provider version across hundreds of workspaces means someone has to
read hundreds of plans. Most are no-ops. A few quietly replace a database or
rotate a credential, and those are easy to miss on plan 150.

tofugov runs the upgrade in every workspace, plans each one, and gives you a
single table sorted by risk, with a one-line summary per workspace. It only
advises: nothing is applied unless you ask for it.

## Install

Requires Go 1.25+, [OpenTofu](https://opentofu.org), and an
[OpenRouter](https://openrouter.ai) API key.

```sh
go install github.com/rchandnaWUSTL/tofugov/cmd/tofugov@latest
```

## Quick start

```sh
export OPENROUTER_API_KEY=sk-or-...
tofugov upgrade --provider hashicorp/aws=6.0.0 ./workspaces/*
```

Output from the bundled fixtures (`make demo`), riskiest first:

```text
ID  WORKSPACE                PLAN         RISK    P_BAD  SUMMARY
6   ws-06-db-credentials     +0 ~0 ±2 -0  HIGH    0.75   Replaces database password and pgpass file due to rotation update
8   ws-08-customer-data      +0 ~0 ±1 -1  HIGH    0.75   Database will be replaced; legacy file deleted.
7   ws-07-schema-migrations  +0 ~0 ±1 -0  MEDIUM  0.30   One null_resource will be replaced due to schema version change.
4   ws-04-checkout-config    +0 ~1 ±0 -0  LOW     0.05   Updates configuration for checkout service to increase replicas to 4.
5   ws-05-feature-flags      +0 ~1 ±0 -0  LOW     0.05   Updates feature flags to enable new checkout flow
1   ws-01-edge-network       +0 ~0 ±0 -0  LOW     0.01   No infrastructure changes planned.
2   ws-02-dns-zones          +0 ~0 ±0 -0  LOW     0.01   No changes to infrastructure.
3   ws-03-logging            +0 ~0 ±0 -0  LOW     0.01   No changes to infrastructure.

Shadow mode: nothing was applied. Scores are recorded, not enforced.
```

Look at one in detail, then apply once you're satisfied:

```sh
tofugov show 6
tofugov upgrade --provider hashicorp/aws=6.0.0 ./workspaces/* --apply
```

`--apply` shows the same table and asks for confirmation first. Pass `--yes` in CI.

## How it works

1. Rewrites the provider (and optionally `required_version`) constraints in each
   workspace's `*.tf` files. Originals are backed up to `<workspace>/.tofugov/backup/`.
2. Runs `tofu init -upgrade` and `tofu plan`, and saves the plan file.
3. Reads the plan JSON into a summary of actions, addresses, and changed attributes.
4. Asks a model for a plain-English explanation and a probability that the
   change goes badly (rollback, incident, or a reviewer rejecting it).
5. Records everything in a local SQLite history and prints the table.

The score never changes what `--apply` does. The point of this stage is to
collect a track record: label what actually happened with `tofugov outcome`, and
`tofugov report` shows how well the scores matched reality.

## Commands

| Command | Description |
|---|---|
| `upgrade <dirs...>` | Upgrade, plan, explain, and score each workspace. `--provider ns/name=constraint` (repeatable), `--tofu-version`, `--apply`, `--yes`, `--on-unscored skip\|proceed` |
| `score <dir>` | Plan, explain, and score one workspace without changing versions. Never applies. |
| `show <id>` | Full explanation, risk rationale, and changed attributes for one recorded change |
| `outcome <id> --result <r>` | Record what happened: `clean`, `rolled_back`, `incident`, or `overridden` |
| `history` | Past changes, newest first. Filter with `--workspace`, `--band`, `--run`, `--unscored` |
| `report` | Predicted risk vs. recorded outcomes, with a Brier score |

Every command accepts `--json`.

## Configuration

| Variable | Flag | Default |
|---|---|---|
| `OPENROUTER_API_KEY` | | required; also read from `./.env` |
| `TOFUGOV_MODEL` | `--model` | `qwen/qwen3-235b-a22b-2507` |
| `TOFUGOV_DB` | `--db` | `.tofugov/tofugov.db` |
| | `--tofu` | `tofu` |
| | `--medium-threshold`, `--high-threshold` | `0.15`, `0.5` |

Any OpenRouter model that returns JSON works, for example `moonshotai/kimi-k2-0905`.

## Safety and privacy

- The model sees a summary of the plan: actions, resource addresses, and changed
  attributes. It never sees your HCL or state file, and values marked sensitive
  are redacted before anything leaves your machine.
- If a plan can't be scored (no key, API error, unusable reply), it's marked
  `UNSCORED`, and `--apply` skips it unless you pass `--on-unscored=proceed`.
- Every replaced or destroyed resource appears in the explanation. If the model
  leaves one out, tofugov adds it.

## Limitations

- Risk scores come from a general-purpose model and are not calibrated. The same
  plan can score differently from run to run. Use them to decide what to read
  first, not what to skip.
- Constraints are only rewritten where a `version` attribute already exists.
- Workspaces are processed one at a time.

## Development

```sh
brew install opentofu go
make test     # unit tests, no network or tofu needed
make demo     # seed 8 local fixture workspaces and run a shadow-mode upgrade
make reset    # restore the fixtures
```

The fixtures in `fixtures/workspaces/` use only the `random`, `local`, and
`null` providers, so they run without cloud credentials. `make seed` applies
them with `seed.tfvars` so the upgrade surfaces real pending changes: no-ops,
in-place updates, a credential rotation, a provisioner re-run, and a database
replacement.

The demo video is a [Remotion](https://www.remotion.dev) project that replays
real terminal recordings:

```sh
scripts/record_video_casts.sh
cd demo/video && npm install && npm run render
```

`docs/prd.md` has the product requirements this prototype implements (Phase 1).
