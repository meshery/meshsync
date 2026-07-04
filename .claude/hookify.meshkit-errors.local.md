---
name: meshkit-errors
enabled: true
event: file
action: block
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: file_path
    operator: not_contains
    pattern: _test.go
  - field: content
    operator: regex_match
    pattern: fmt\.Errorf\(|errors\.New\(\s*"|errors\.Errorf\(|errors\.Wrapf?\(
---

MeshKit error framework required for every error.

This `.go` edit adds an ad-hoc error (`fmt.Errorf`, std-lib `errors.New("…")`, or `pkg/errors`) that bypasses MeshKit. Every error MUST be a structured MeshKit error with a unique code (AGENTS.md Critical Rule 1).

Replace it with a builder over `github.com/meshery/meshkit/errors`:

```go
const ErrFooCode = "NNNN"
func ErrFoo(err error) error {
    return errors.New(
        ErrFooCode, errors.Alert,
        []string{"short description"},
        []string{err.Error()},
        []string{"probable cause(s)"},
        []string{"remedy/remedies"},
    )
}
```

Allocate the next free code from `helpers/component_info.json`'s `next_error_code` in the package's `error.go`. MeshKit's own `errors.New(ErrCode, …)` is allowed - its first argument is a code constant, not a string literal. `.github/workflows/error-codes-updater.yml` verifies and self-corrects codes on push to `master`.

Companion to `.claude/hooks/meshkit-errors.sh` (the wired, net-new-aware enforcer). Fires when this directory is the active working directory.
