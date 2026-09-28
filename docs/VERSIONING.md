# Versioning

NeoNect uses [Semantic Versioning](https://semver.org/) (SemVer) with the format:

**MAJOR.MINOR.PATCH**

- **MAJOR**: Breaking changes to the public API, protocol invariants, or storage compatibility.
- **MINOR**: Backward-compatible new functionality and features.
- **PATCH**: Backward-compatible bug fixes and security patches.

## First Stable Release
`v1.0.0` is the first stable release identifier for NeoNect Server, representing the finalized blind-relay architecture and operational configuration limits.

## Git Workflow vs. Releases
- **Branch**: An ongoing line of development (e.g., `main` or `dev`). A branch is NOT a release version.
- **Commit**: A specific snapshot in the Git history. A commit is NOT a release.
- **Tag**: An immutable pointer to a specific commit used strictly for version identification (e.g., `v1.0.0`).
- **GitHub Release**: The packaged, public-facing software release associated with a Git Tag, containing change logs and built binaries if applicable.
