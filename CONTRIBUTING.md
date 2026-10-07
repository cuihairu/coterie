# Contributing to Coterie

Thanks for your interest in contributing! Coterie is in early development, so
design feedback, domain-model review, and issue discussion are especially
valuable right now.

## How to contribute

1. Open or comment on an
   [issue](https://github.com/cuihairu/coterie/issues) — bug reports, feature
   proposals, and domain-model questions are all welcome.
2. For code changes, fork the repo and create a branch from `main`.
3. Keep PRs focused: one logical change per PR.
4. Make sure `go build ./... && go test ./...` passes before submitting.

Issues and PRs in English or 中文 are both welcome.

## Project layout

- Intended module structure: [docs/design.md · 模块划分](docs/design.md#8-模块划分)
- What Phase 1 must deliver: [docs/requirements.md](docs/requirements.md)

## Code style

- Follow standard Go conventions: `gofmt`, `go vet`.
- Domain concepts must match the terms defined in
  [docs/design.md](docs/design.md). If you need a new concept or want to change
  an existing one, propose it in an issue first — the domain model is the
  contract.

## License

By contributing, you agree that your contributions will be licensed under the
[Apache License 2.0](LICENSE).
