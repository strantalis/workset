# Deprecation Register

The source of truth is [`deprecation-register.yaml`](./deprecation-register.yaml).

Use this register for compatibility shims and legacy paths that must be removed later.

The active register is currently clear. The entries below are kept only as completed records for the workset hard cut.

## Required fields

- `id`: stable identifier (do not rename once published).
- `scope`: affected surface area (for example `global-config`, `api-config`, `frontend`).
- `summary`: what compatibility behavior exists and why.
- `introduced`: date the compatibility path was introduced (`YYYY-MM-DD`).
- `remove_by`: date this must be removed or explicitly extended (`YYYY-MM-DD`).
- `owner`: responsible team or owner handle.
- `tracking_issue`: issue/doc path for cleanup work.
- `status`: `active` or `completed`.
- `evidence`: files proving where the compatibility behavior exists.

## Enforcement

Validation runs in:

- `make deprecations`
- `.github/workflows/lint.yml` (`deprecation-register` job)

The validator fails CI when:

1. required metadata is missing or invalid, or
2. an `active` entry passes its `remove_by` date.

When cleanup is intentionally deferred, update `remove_by` and link a tracking issue in the same PR.
