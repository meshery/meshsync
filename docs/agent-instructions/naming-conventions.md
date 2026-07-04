# Identifier Naming Conventions

## The Ecosystem Contract

**Wire is camelCase everywhere; DB is snake_case; Go fields follow Go idiom; the ORM layer is the sole translation boundary.**

- Authoritative source: `meshery/schemas/AGENTS.md § Casing rules at a glance`
- Reader-friendly directory: <https://github.com/meshery/schemas/blob/master/docs/identifier-naming-contributor-guide.md>
- The contract is not optional ecosystem-wide; deviations in schema-driven repos block PRs via the schemas consumer-audit CI gate.
- `Id` (camelCase), never `ID`, in URL params, JSON tags, and TypeScript properties.

## MeshSync's Divergence

Unlike `meshery-cloud` (fully schema-driven), MeshSync's canonical model - `pkg/model.KubernetesResource` and its siblings (`KubernetesResourceObjectMeta`, `KubernetesResourceSpec`, `KubernetesResourceStatus`, `KubernetesKeyValue`) - is a **local Go/GORM struct**, not generated from `github.com/meshery/schemas`. It predates the ecosystem-wide schemas migration and has never been brought into it.

The model already **mixes casing**: most JSON tags are camelCase (`apiVersion`, `resourceVersion`, `generateName`), but several are snake_case (`cluster_id`, `unique_id`, `pattern_resource`, `component_metadata`) and a few are lowercase-run-together legacy forms. This is known, load-bearing technical debt - Meshery Server and other consumers depend on the exact wire shape today.

**Rules:**

- New fields added to any `pkg/model` struct MUST use camelCase JSON tags, per the ecosystem contract - do not add another snake_case field to match the existing bad examples.
- Do NOT opportunistically recase an existing field (e.g. `cluster_id` -> `clusterId`) as a drive-by cleanup. That is a breaking wire change for every consumer of the NATS stream (Meshery Server) and any persisted snapshot files; it requires an explicit, coordinated migration (bump a version, update all consumers in the same change, or introduce a new field alongside the old one with a deprecation path) - not a silent rename.
- If a future change migrates `pkg/model` into `github.com/meshery/schemas` (bringing MeshSync fully into the schema-driven pattern used by `meshery-cloud`), that migration is the point to resolve the mixed casing deliberately, and it must run `cd ../schemas && make validate-schemas && make consumer-audit` and follow `schemas/AGENTS.md`'s Dual-Schema Pattern.
