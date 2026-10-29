# Release preparation

This implementation has an Unreleased changelog; no release/tag publication or
remote push is authorized by local development. The existing [MIT license](../LICENSE)
is preserved. Go 1.27.1 is the minimum currently verified by the module and tests.

Before preparing a release:

1. Review [delivery evidence](DELIVERY_CHECKLIST.md), provider unknowns, and the
   final diff. Verify credentials and sensitive data are absent.
2. Run formatting, `go test ./...`, `go test -race ./...`, `go vet ./...`, and
   `go build ./examples/...` on every claimed supported Go version.
3. Confirm CI succeeds on the intended release commit. Local checks do not prove
   the remote workflow has run.
4. Review public API compatibility, safe error behavior, token concurrency, and
   service submission counts. Include relevant changelog entries and docs.
5. Decide the version and move implemented Unreleased changes under that version
   only when the release is actually made. Do not fabricate release dates/tags.
6. Obtain explicit owner authorization for remote pushes, tag creation/publication,
   and release publication before carrying out those actions.

Use Semantic Versioning. Begin with a maintainer-selected `v0.x.y` if the API is
still provisional; incompatible changes must be identified clearly. For a stable
`v1`, incompatible public API changes require a major version. Go module major
versions `v2` and later require a matching `/v2` (etc.) module/import path, updated
imports and examples, and matching version tags. Versions zero and one use the
current unsuffixed module path.

Document the supported-release/security policy when it exists. No live billable
operation is required or authorized by these checks. Live provider compatibility
must be established separately without presenting mock tests as proof. Retain
uncertainties about token overlap, final delivery, and idempotency in release notes.
