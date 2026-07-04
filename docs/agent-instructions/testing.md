# Testing

MeshSync has three tiers: unit tests, integration tests against a real kind cluster, and runtime verification of a running agent. Prefer the narrowest tier that actually exercises the changed code path.

## Unit Tests

- `make test` - runs `lint-run` first, then `go test -failfast --short ./... -race`.
- `make lint-run` - `golangci-lint run ./...` on its own.
- Single package: `go test ./internal/pipeline/... -race`
- Single test: `go test ./internal/pipeline/... -run TestName -count=1`
- `make coverage-report` - `go test -v ./... -coverprofile cover.out` then renders `cover.html`.
- `make mod-tidy` - `go mod tidy`; run after any dependency change.

## Integration Tests

- `make integration-tests` runs the full cycle: `integration-tests-check-dependencies` (docker, kind, kubectl present) -> `integration-tests-setup` (`integration-tests/infrastructure/setup.sh setup` - NATS via docker-compose plus a kind cluster) -> `integration-tests-run` (builds the binary, then `RUN_INTEGRATION_TESTS=true MESHSYNC_BINARY_PATH=<abs path> SAVE_MESHSYNC_OUTPUT=true go test -v -count=1 -run Integration ./integration-tests`) -> `integration-tests-cleanup` (tears down docker-compose and the kind cluster).
- Test files live directly under `integration-tests/` (not a nested Go package per scenario): binary-mode broker and file-output scenarios, library-mode custom-broker scenarios, and manifest-unmarshaling cases. Add a new case to the existing scenario file for its mode rather than creating a new top-level test file.
- Run `integration-tests-setup` and `integration-tests-cleanup` in pairs - a leaked kind cluster or docker-compose stack from a prior failed run will make the next `integration-tests-run` misbehave; when in doubt, run `integration-tests-cleanup` before `integration-tests-setup`.

## Runtime Verification

Tests prove behavior in isolation; they do not prove the discovery pipeline actually fires correctly against a live cluster (informer resync, filtering, backoff/reconnect). For that, use the `verifier-meshsync` skill (`.claude/skills/verifier-meshsync/`), which scripts standing up an isolated local cluster, running the built binary in file mode with debug logging, and reading the evidence out of the log and output file. Reach for it whenever a change touches discovery, filtering, or config, or whenever `/verify` runs on this repo.

## CI

- `.github/workflows/ci.yml` - lint + unit tests on PRs.
- `.github/workflows/integration-tests-ci.yml` - the integration-test cycle.
- `.github/workflows/error-codes-updater.yml` - runs on push to `master` for `**.go` changes; see [errors](errors.md).
