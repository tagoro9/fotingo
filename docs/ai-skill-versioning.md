# Skill Versioning

`skillMetadataVersion` in `internal/commands/ai/skills.go` uses semantic versioning (`MAJOR.MINOR.PATCH`).

## How To Bump

1. Update `skillMetadataVersion` in `internal/commands/ai/skills.go`.
2. Use these bump rules:
   - `PATCH`: wording/example refinements that do not change meaning or required structure.
   - `MINOR`: backward-compatible additions to skill guidance/frontmatter.
   - `MAJOR`: breaking changes to expected skill structure or semantics.
3. Regenerate expectations by updating goldens under `internal/commands/ai/testdata/`.
4. Run `go test ./internal/commands/ai ./pkg/commands`.
