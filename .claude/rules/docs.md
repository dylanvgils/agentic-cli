---
paths:
  - "docs/**"
  - "README.md"
  - "zensical.toml"
---
# Docs rules

- User-facing changes update the matching `docs/` page. Touch `README.md` only for what it covers.
- A new page gets a line in:
  - `docs/README.md`.
  - The `README.md` index.
  - The `nav` in `zensical.toml`.
- Each fact lives on one page; others link in one line. Grep `docs/` first.
- Paragraphs about 3 sentences, table cells one; bullets for conditions and steps.
- No internals users can't act on; contributor details go in `docs/development.md` or the root `CLAUDE.md`.
- Never condense or rewrite personal sections such as Motivation in `docs/overview.md`.
