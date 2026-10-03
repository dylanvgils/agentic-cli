# Contributing

## Setup

Go and Make are required. Run `make build` to compile and `make test` to run the tests.

```bash
make build   # compile to bin/agentic
make test    # run unit tests
```

If you don't have Go installed, `make docker-dist` builds everything via Docker.

## Making changes

- Add tests for new code - see `CLAUDE.md` or `docs/development.md` for conventions
- Keep the matching `docs/` page in sync with any user-facing behaviour changes; `README.md` is an overview only

## Pull requests

PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add opencode support
fix: correct mount path expansion
docs: update configuration table
```

Allowed types: `feat`, `fix`, `chore`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `revert`.
