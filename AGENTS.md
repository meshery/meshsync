# MeshSync

> `CLAUDE.md` is a symlink to `AGENTS.md`; they are always identical. Edit `AGENTS.md`.

MeshSync is Meshery's Kubernetes cluster-state synchronization agent: a Go controller (not a Kubebuilder operator) that watches a cluster with dynamic informers, converts every discovered resource into a canonical model, and publishes it to Meshery Server over NATS (or to a file). Meshery Operator deploys one MeshSync instance per managed cluster (see `meshery/meshery-operator`'s `MeshSync` CRD) and injects `BROKER_URL`.

## Commands

- `make build` - compile `main.go` to `bin/meshsync`
- `make run` - start a local NATS container (`make nats-run`) then run MeshSync against it with `DEBUG=true`
- `make test` - `lint-run` then `go test -failfast --short ./... -race`
- `make lint-run` - `golangci-lint run ./...`
- `make coverage-report` - unit tests with an HTML coverage report (`cover.html`)
- `make integration-tests` - full kind + NATS docker-compose cycle (`integration-tests-setup` -> `integration-tests-run` -> `integration-tests-cleanup`); see [testing](docs/agent-instructions/testing.md) for sub-targets and single-test forms
- Runtime verification against a live cluster (not just tests): use the `verifier-meshsync` skill (`.claude/skills/verifier-meshsync/`)

## Critical Rules

1. **MeshKit structured errors only.** Every error is a builder over `github.com/meshery/meshkit/errors` with a unique code constant (`^Err[A-Z].+Code$`) - never `fmt.Errorf`/`errors.New("...")`. Codes are allocated from `helpers/component_info.json`'s `next_error_code` and are unique per package `error.go` (`meshsync/error.go`, `internal/pipeline/error.go`, `internal/config/error.go`). `.github/workflows/error-codes-updater.yml` runs `meshkit/cmd/errorutil` on every push to `master` touching `**.go` and self-commits updated codes/exports - do not hand-allocate a code that utility would reassign. Full convention: [errors](docs/agent-instructions/errors.md).
2. **Identifier naming - partial.** Unlike `meshery-cloud`, MeshSync's wire model (`pkg/model.KubernetesResource` and siblings) is a local Go/GORM struct, not sourced from `github.com/meshery/schemas`, and already mixes `camelCase` and `snake_case` JSON tags. New fields follow the ecosystem's camelCase-wire contract; do not silently recase existing fields. Full rule and the migration path if this model is ever moved into schemas: [naming conventions](docs/agent-instructions/naming-conventions.md).
3. **Deployment coupling.** MeshSync's CLI flags, config schema (`internal/config`), and the `KubernetesResource` model are consumed by Meshery Operator (which deploys this binary) and Meshery Server (which consumes the broker stream). Coordinate flag/config/model changes with both repos; do not assume this repo alone defines the contract.
4. **Pipeline stages are rebuilt per run.** `internal/pipeline.New` constructs fresh stages on every discovery and resync; never hoist stages or steps into shared package-level state - see [architecture](docs/agent-instructions/architecture.md).

## Required on Every PR

- **Tests accompany every behavioral change.** Run every locally-runnable test
  before requesting review; never defer runnable coverage to reviewers or
  follow-up PRs.
- **Documentation accompanies every behavioral change, in both forms:**
  - External, user-facing: docs.meshery.io (source: meshery/meshery docs) -
    update whenever the change is user-visible.
  - Internal, developer-facing: this repo's [`docs/`](docs/) - update whenever
    architecture, workflows, or contracts change.
- **Schema-aware changes**: MeshSync's wire model is not yet sourced from `meshery/schemas` (see [naming conventions](docs/agent-instructions/naming-conventions.md)); if a change migrates or aligns a field with the schemas contract, run `cd ../schemas && make validate-schemas && make consumer-audit` before pushing.
- **Sign off every commit** (`git commit -s`).
- **No AI attribution** in commits, PR descriptions, comments, or code.

## Detailed Instructions

- [Architecture](docs/agent-instructions/architecture.md) - discovery pipeline, output/dedup, channels, config, deployment topology
- [Errors](docs/agent-instructions/errors.md) - MeshKit error convention and the errorutil workflow
- [Naming conventions](docs/agent-instructions/naming-conventions.md) - the ecosystem identifier-casing contract and MeshSync's current divergence
- [Testing](docs/agent-instructions/testing.md) - unit, integration, and runtime-verification commands

## Claude Automation

`.claude/hooks/meshkit-errors.sh` blocks net-new ad-hoc errors (`fmt.Errorf`/`errors.New("...")`) in any edited `.go` file - see Critical Rule 1. `.claude/hooks/no-ai-attribution.sh` blocks AI-attribution content in any tool call. `.claude/hooks/session-start.sh` provisions `../meshery-operator` as a sibling on remote (web) sessions for the cross-repo checks in Critical Rule 3. `.claude/skills/verifier-meshsync/` is the runtime-verification skill referenced under Commands.

## Further Reading

- [design-spec: MeshSync infrastructure synchronization](docs/design-spec_meshsync-infrastructure-synchronization.md)
- [design-spec: embedded MeshSync](docs/design-spec_embedded-meshsync.md)
