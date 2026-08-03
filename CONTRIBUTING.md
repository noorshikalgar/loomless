# Contributing

## Branching strategy — GitHub Flow

This repo uses [GitHub Flow](https://docs.github.com/en/get-started/using-github/github-flow):
a single long-lived branch, everything else is short-lived.

- **`main` is always deployable / releasable.** Direct commits to `main` are
  reserved for the maintainer during initial scaffolding; once Phase 0 code
  lands, `main` is protected and every change goes through a pull request.
- **Branch off `main` for every change**, named by type:
  - `feature/<short-description>` — new functionality
  - `fix/<short-description>` — bug fixes
  - `docs/<short-description>` — documentation-only changes
  - `chore/<short-description>` — tooling, CI, repo maintenance
- **Open a PR early**, even as a draft, so the direction is visible before
  it's finished.
- **PRs require at least one review** before merging once there's more than
  one contributor. Squash-merge into `main` to keep history linear and
  bisectable.
- **Delete the branch after merge.**
- **Tag releases** off `main` using [SemVer](https://semver.org/)
  (`v0.1.0`, `v0.2.0`, ...) once there's a first working build (Phase 0 exit
  criteria — see [`docs/product/09-roadmap.md`](docs/product/09-roadmap.md)).

No `develop` branch, no long-lived release branches — the roadmap phases in
`docs/product/09-roadmap.md` are the sequencing; branches stay short-lived
regardless of which phase a change belongs to.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org/): `type(scope):
summary`, e.g. `feat(cli): add story record start command`. Common types:
`feat`, `fix`, `docs`, `chore`, `refactor`, `test`.

## Before opening a PR

- Explain *why*, not just *what*, in the PR description — link to the
  relevant doc/open question if the change touches a product decision.
- Keep PRs scoped to one logical change.

## Product direction

This project isn't open for feature contributions yet — the core product
decisions in [`docs/product/10-open-questions.md`](docs/product/10-open-questions.md)
haven't been resolved. Docs/scaffolding PRs are welcome; open an issue to
discuss before starting on capture-engine code.
