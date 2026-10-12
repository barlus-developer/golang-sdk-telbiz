## Behavior

Describe the concrete problem and resulting behavior. Include public API changes.

## Validation

- [ ] Go source formatted
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] `go build ./examples/...`
- [ ] Documentation and examples updated

List any blocked checks and why. Mock tests do not establish live Telbiz compatibility.

## Authentication and service impact

Explain changes to renewal, concurrent access, cancellation, or Close. Confirm
whether service submission count can change and that billable POSTs are not
automatically replayed.

## Release notes

State compatibility impact and relevant Unreleased changelog entry. Remove
credentials, tokens, real recipients, and SMS/OTP content from the entire PR.
