# Release

## GitHub Release Artifacts

`wacli-reader` uses GoReleaser (`.goreleaser.yaml` for macOS, `.goreleaser-linux-windows.yaml` for linux/windows) and the GitHub Actions workflow `.github/workflows/release.yml`.

To cut a release:

1. Tag and push:
   - `git tag vX.Y.Z`
   - `git push origin vX.Y.Z`
2. Wait for the GitHub Actions “release” workflow to publish the release artifacts.

To re-release an existing tag, run the workflow manually and pass the tag (e.g. `v0.0.2`).

Expected macOS artifact name:

- `wacli-reader-macos-universal.tar.gz`

Other artifacts:

- `wacli-reader-linux-<arch>.tar.gz`
- `wacli-reader-windows-<arch>.zip`

Remember to also bump the `version` constant in [`cmd/wacli/root.go`](../cmd/wacli/root.go) so `wacli-reader version` matches the new tag, and add a `## [X.Y.Z]` entry to [`CHANGELOG.md`](../CHANGELOG.md).
