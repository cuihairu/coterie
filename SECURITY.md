# Security Policy

## Reporting a vulnerability

Please do **not** report security vulnerabilities through public GitHub
issues.

- Use GitHub's private security advisories (**Report a vulnerability** on the
  repository's Security tab), or
- email **chuihairu@gmail.com** with the details.

Please include steps to reproduce, the affected version/commit, and any impact
assessment you can provide. Reports in English or 中文 are both welcome.

## Scope

Coterie handles membership, billing, and (optionally) credential-metadata data.
Areas of particular interest:

- authentication and session handling
- authorization on coterie / subscription resources
- the secret store — it must never persist plaintext credentials in core
  tables (see [docs/design.md · 安全与 Secret](docs/design.md#6-安全与-secret))

## Supported versions

The project has not shipped a release yet; security fixes apply to `main`.

## Response time

Maintainers aim to acknowledge reports within 7 days.
