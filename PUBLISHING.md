# Publishing to GitHub

This repo is local-only right now (see the "GitHub remote" note in
[`docs/product/11-decisions.md`](docs/product/11-decisions.md)). These
are the steps to run **yourself**, whenever you're ready — nothing here
runs automatically.

## 1. Create the repo

Pick one:

- **Web:** [github.com/new](https://github.com/new) — name it
  `loomless`, choose public/private, and **don't** initialize with a
  README/license/.gitignore (this repo already has all three; letting
  GitHub add its own would conflict on first push).
- **CLI:**
  ```bash
  gh repo create noorshikalgar/loomless --public --source=. --remote=origin
  ```
  (`--private` instead of `--public` if you'd rather start closed.) This
  also adds the `origin` remote for you — skip step 2 if you use this.

## 2. Add the remote and push (skip if `gh repo create --source=.` did it)

```bash
git remote add origin git@github.com:noorshikalgar/loomless.git
git push -u origin main
```

## 3. Optional polish once it's up

- Repo page → About: add a short description and topics (`cli`,
  `developer-tools`, `mcp`, `go`, `macos`) so it's discoverable.
- Branch protection on `main` if you want PRs required going forward —
  see [`CONTRIBUTING.md`](CONTRIBUTING.md) for the branching model this
  repo already follows.
- Once external contributions actually open, update the "Contributing"
  section in [`README.md`](README.md) to point at the live repo instead
  of saying "not yet open."

## Sanity check before pushing

The `.gitignore` already excludes `.env*`, `.loomless/` session data, and
build artifacts — worth a quick `git status`/`git log --stat` skim before
the first push just to confirm nothing local (a stray API key, a real
session with your own screenshots) snuck into a commit.
