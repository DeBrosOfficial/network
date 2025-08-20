# Contributing to DeBros Network

Thanks for helping improve the network! This guide covers setup, local dev, tests, and PR guidelines.

## Requirements

- Go 1.22+ (1.23 recommended)
- RQLite (optional for local runs; the Makefile starts nodes with embedded setup)
- Make (optional)

## Setup

```bash
git clone https://git.debros.io/DeBros/network.git
cd network
make deps
```

## Build, Test, Lint

- Build: `make build`
- Test: `make test`
- Format/Vet: `make fmt vet` (or `make lint`)

```

Useful CLI commands:

```bash
./bin/network-cli health
./bin/network-cli peers
./bin/network-cli status
```

## Versioning

- The CLI reports its version via `network-cli version`.
- Releases are tagged (e.g., `v0.18.0-beta`) and published via GoReleaser.

## Pull Requests

1. Fork and create a topic branch.
2. Ensure `make build test` passes; include tests for new functionality.
3. Keep PRs focused and well-described (motivation, approach, testing).
4. Update README/docs for behavior changes.

Thank you for contributing!
