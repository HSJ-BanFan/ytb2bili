# Domain Docs

Engineering skills should consume this repository's domain documentation as follows.

## Before exploring

- Read `CONTEXT.md` at the repository root if it exists.
- Read relevant ADRs under `docs/adr/`.
- If these files do not exist, proceed silently. Create them lazily when domain terms or decisions are resolved.

## Layout

This is a single-context repository:

- `CONTEXT.md` — domain glossary and model
- `docs/adr/` — architecture decision records

## Domain vocabulary

Use terminology defined in `CONTEXT.md` for issue titles, refactor proposals, hypotheses, and test names. If a needed concept is missing, consider whether it should be added through domain modeling.

## ADR conflicts

If an output contradicts an existing ADR, call out the conflict explicitly instead of silently overriding it.
