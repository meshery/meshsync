#!/usr/bin/env bash
# PreToolUse guard - enforce MeshKit-style structured errors.
#
# MeshSync mandates the MeshKit errors framework for every error (AGENTS.md
# Critical Rule 1, "MeshKit structured errors only"). A MeshKit error is a
# builder over a unique code constant:
#
#   import "github.com/meshery/meshkit/errors"
#   const ErrFooCode = "NNNN"
#   func ErrFoo(err error) error {
#       return errors.New(
#           ErrFooCode, errors.Alert,
#           []string{"short description"},
#           []string{err.Error()},
#           []string{"probable cause(s)"},
#           []string{"remedy/remedies"},
#       )
#   }
#
# The anti-patterns are ad-hoc errors that bypass the framework: fmt.Errorf,
# the std-lib errors.New("literal"), and pkg/errors (errors.Errorf / errors.Wrap
# / errors.Wrapf). MeshKit's own errors.New is NOT matched because its first
# argument is a code-constant identifier, never a string literal.
#
# This is the EARLY, in-session catch (it only governs Claude Code tool calls).
# It flags only NET-NEW introductions, so editing or migrating existing ad-hoc
# errors is never blocked - only adding a new one is. The authoritative,
# environment-independent enforcement is .github/workflows/error-codes-updater.yml
# (runs meshkit/cmd/errorutil on push to master) plus human review on the PR.
#
# Contract: reads the PreToolUse JSON payload on stdin; exits 2 (deny, message
# shown to the agent) when the edited path is a non-test/mock/generated .go
# file and the edit adds more ad-hoc-error occurrences than it removes.
# Anything else passes (exit 0). MeshSync has no ui/server split - every .go
# file in the repo is backend code, so match broadly and rely on the
# test/mock/generated exclusion below.
set -uo pipefail

command -v jq >/dev/null 2>&1 || exit 0 # no jq → fail open, CI still enforces

payload="$(cat)"
tool="$(jq -r '.tool_name // empty' <<<"$payload")"
path="$(jq -r '.tool_input.file_path // empty' <<<"$payload")"

case "$tool" in
  Edit | Write | MultiEdit) ;;
  *) exit 0 ;;
esac
case "$path" in
  *.go) ;;
  *) exit 0 ;;
esac
# Exclude tests, mocks, and generated sources: these legitimately use fmt/std errors.
case "$path" in
  *_test.go | *_mock.go | *mock_*.go | *.gen.go | *.pb.go) exit 0 ;;
esac

# Text being ADDED (Write.content, Edit.new_string, MultiEdit.edits[].new_string)
new="$(jq -r '[ .tool_input.content, .tool_input.new_string, (.tool_input.edits[]?.new_string) ]
              | map(select(. != null)) | join("\n")' <<<"$payload")"
# Text being REMOVED/replaced - so a pure migration away from ad-hoc errors, or
# an unrelated edit to a line that already contained one, is never flagged.
old="$(jq -r '[ .tool_input.old_string, (.tool_input.edits[]?.old_string) ]
              | map(select(. != null)) | join("\n")' <<<"$payload")"

# Ad-hoc (non-MeshKit) error constructors. errors.New([[:space:]]*") targets the
# std-lib string-literal form only; MeshKit's errors.New(ErrCode, …) never matches.
ADHOC='fmt\.Errorf\(|errors\.New\([[:space:]]*"|errors\.Errorf\(|errors\.Wrapf?\('

count() { printf '%s' "$1" | grep -oE "$ADHOC" 2>/dev/null | grep -c . ; }
new_count="$(count "$new")"
old_count="$(count "$old")"

if [ "$new_count" -gt "$old_count" ]; then
  offenders="$(printf '%s' "$new" | grep -nE "$ADHOC" 2>/dev/null | sed 's/^/    /' | head -12)"
  {
    echo "⛔ MeshKit error guard - ${path##*/}"
    echo
    echo "This edit introduces $((new_count - old_count)) ad-hoc error(s) that bypass the"
    echo "MeshKit errors framework. MeshSync requires every error to be a structured"
    echo "MeshKit error with a unique code (AGENTS.md Critical Rule 1)."
    echo
    echo "Flagged in the added text:"
    echo "$offenders"
    echo
    echo "Replace each with a MeshKit error builder:"
    echo "  const ErrFooCode = \"NNNN\""
    echo "  func ErrFoo(err error) error {"
    echo "      return errors.New("
    echo "          ErrFooCode, errors.Alert,"
    echo "          []string{\"short description\"},"
    echo "          []string{err.Error()},"
    echo "          []string{\"probable cause(s)\"},"
    echo "          []string{\"remedy/remedies\"},"
    echo "      )"
    echo "  }"
    echo
    echo "Import: github.com/meshery/meshkit/errors . Allocate the next free code from"
    echo "helpers/component_info.json's next_error_code in the package's error.go - "
    echo "error-codes-updater.yml verifies and self-corrects codes on push to master."
    echo
    echo "Note: this guard only blocks NET-NEW ad-hoc errors. Migrating an existing"
    echo "fmt.Errorf to MeshKit (removing as many as you add) passes cleanly."
  } >&2
  exit 2
fi

exit 0
