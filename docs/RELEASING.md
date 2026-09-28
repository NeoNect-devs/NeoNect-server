# Releasing

This document outlines the required maintainer workflow to create a stable release.

*Note: A commit is not a release. A tag identifies the version, and a GitHub Release distributes it.*

## Maintainer Workflow

1. **Verify Branch**: Ensure you are on the main release branch (`main`).
2. **Verify Working Tree**: Ensure your working tree is clean and all PRs are merged.
3. **Run Tests**:
   ```bash
   go test -count=1 ./...
   ```
4. **Run Static Checks**:
   ```bash
   go vet ./...
   ```
5. **Run Diff Checks**:
   ```bash
   git diff --check
   ```
6. **Verify VERSION**: Open the `VERSION` file and ensure it contains the exact target SemVer number (e.g., `1.0.0`).
7. **Update CHANGELOG**: Ensure `CHANGELOG.md` reflects the changes for this specific version.
8. **Commit**: Create the release preparation commit.
   ```bash
   git commit -m "docs(release): prepare v1.0.0"
   ```
9. **Create Annotated Tag**: Create the tag using the `vX.Y.Z` format.
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   ```
10. **Push Commit**:
    ```bash
    git push origin main
    ```
11. **Push Tag**:
    ```bash
    git push origin v1.0.0
    ```
12. **Create GitHub Release**: Navigate to GitHub, draft a new release pointing to the pushed tag, and attach the release notes.
13. **Verify Remote**: Ensure the remote tag and branch point to the exact expected commit hash.
