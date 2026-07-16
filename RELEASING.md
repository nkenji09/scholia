# Releasing

1. Land reviewed changes on `main`.
2. If this release includes changes under `agents/` (skills), bump `"version"` in
   `agents/.claude-plugin/plugin.json` to match the release tag (`vX.Y.Z` → `X.Y.Z`).
   If the release has no skill changes, leave `plugin.json` as-is.
   The Go binary's version is injected from the git tag via goreleaser `ldflags`
   (`internal/cli.version`), so no code change is needed for that.
3. Tag the release: `git tag vX.Y.Z && git push origin vX.Y.Z`.
4. CI (`.github/workflows/release.yml`) runs `goreleaser release --clean` on the tag push,
   producing a **draft** GitHub Release with binaries and an auto-generated changelog
   (commit log since the previous tag — see `.goreleaser.yaml`; no hand-written
   `CHANGELOG.md` is maintained).
5. Review the draft on GitHub and Publish.

See [`agents/.claude-plugin/plugin.json`](agents/.claude-plugin/plugin.json) sync policy background:
`pmem decision list --on tag:req.skill-distribution`.
