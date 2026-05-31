# Contributing

## Setup

Go and Make are required. Run `make build` to compile and `make test` to run the tests.

```bash
make build   # compile to bin/agentic
make test    # run unit tests
```

If you don't have Go installed, `make docker-dist` builds everything via Docker.

## Making changes

- Add tests for new code - see `CLAUDE.md` or `docs/05-development.md` for conventions
- Keep `README.md` in sync with any user-facing behaviour changes

## Pull requests

PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add opencode support
fix: correct mount path expansion
docs: update configuration table
```

Allowed types: `feat`, `fix`, `chore`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `revert`.
