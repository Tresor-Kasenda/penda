# Release Guide

This document describes the release process for `penda`.

## 1. Pre-release Checklist

- Ensure `go test ./...` passes
- Ensure `go vet ./...` passes
- Ensure docs are updated (`docs/`, `README.md`)
- Ensure examples run (`examples/hello`, `examples/rest-api`, `examples/web-app`)
- Review public API changes and migration notes
- Update `CHANGELOG.md`

## 2. Versioning Policy

- Pre-1.0: breaking changes are allowed but must be documented in `MIGRATION.md`
- 1.x+: SemVer
  - `MAJOR`: breaking API change
  - `MINOR`: backward-compatible features
  - `PATCH`: bug fixes and internal improvements

## 3. Release Steps (Git)

1. Create a release branch (optional, depending on workflow)
2. Update version references in docs/examples if needed
3. Update `CHANGELOG.md`
4. Commit release prep changes
5. Tag release:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z"
```

6. Push branch and tag:

```bash
git push origin <branch>
git push origin vX.Y.Z
```

## 4. Post-release Steps

- Create GitHub release notes from `CHANGELOG.md`
- Announce any breaking changes and link `MIGRATION.md`
- Start a new `Unreleased` section in `CHANGELOG.md`

## 5. Module Path / Publishing Note

Current module path is `penda` for local learning/dev work.

Before public release, switch `go.mod` to a real import path, for example:
- `github.com/<org>/penda`

Then update:
- docs snippets
- examples imports
- install instructions
