# Errors

MeshSync uses MeshKit's structured error framework exclusively - never `fmt.Errorf`, std-lib `errors.New("...")`, or `pkg/errors`.

## Convention

- One exported code constant per error, matching `^Err[A-Z].+Code$` (e.g. `ErrGetObjectCode = "1004"`), plus one constructor:

  ```go
  func ErrGetObject(err error) error {
      return errors.New(
          ErrGetObjectCode, errors.Alert,
          []string{"Error getting config object"},
          []string{err.Error()},
          []string{"Config doesnt exist"},
          []string{"Check application config is configured correct or restart the server"},
      )
  }
  ```

- Codes are unique across the whole component, not just the package. Current registries: `meshsync/error.go` (1004-1013), `internal/pipeline/error.go`, `internal/config/error.go`.
- Keep the short-description/probable-cause/remediation string literals as literals (the errorutil tool extracts them for the generated reference); the dynamic cause (`err.Error()`) goes only in the long description slot.

## The `errorutil` Workflow

- `helpers/component_info.json` tracks `next_error_code` for this component (`"name": "meshsync", "type": "controller"`) - allocate the next code from there when adding an error.
- `.github/workflows/error-codes-updater.yml` runs on every push to `master` that touches `**.go`: it executes `go run github.com/meshery/meshkit/cmd/errorutil -d . update --skip-dirs meshery -i ./helpers -o ./helpers`, which normalizes placeholder codes, bumps `next_error_code`, and writes `helpers/errorutil_errors_export.json` and related analysis artifacts, self-committing the result to `master`.
- A second step in the same workflow pushes `helpers/errorutil_errors_export.json` into `meshery/meshery`'s `docs/_data/errorref/meshsync_errors_export.json` - this is the source of the MeshSync error-code reference on docs.meshery.io. Do not hand-edit that file in either repo.
- Because the utility self-commits, do not hand-allocate or hand-renumber a code in a PR if you can instead push a placeholder and let the workflow assign it on merge; if you must allocate locally (e.g. to write a test against a specific code), expect the workflow to potentially renumber it after merge.
