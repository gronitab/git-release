# git-release

A git plugin for opinionated semver release management with first-class release candidate support.

Installed as `git-release`, it becomes available as `git release`.

---

## Motivation

Every project ends up with a bespoke `Makefile` or `scripts/release.sh` that solves the same
problem imperfectly. `git-release` standardises the workflow:

- Releases are always **semver tags** pushed to the remote.
- **Release candidates** (`-rc`, `-rc.2`, …) represent staged/pre-production builds.
- The tool reads the remote tag state before every operation, so it is safe to run from any
  machine and always operates on current truth.

---

## Installation

```sh
curl -fsSL https://github.com/gronitab/git-release/releases/latest/download/install.sh | sh
```

Places `git-release` in `/usr/local/bin` — git picks it up automatically as `git release`.

To install to a different directory: `INSTALL_DIR=~/bin sh <(curl -fsSL ...)`

### Updating

```sh
git release update
```

---

## Concepts

### Production release tag
A plain semver tag with no pre-release suffix: `v1.2.3`.
The **latest prod tag** is the highest such tag by semver ordering.

### Release candidate (RC) tag
A tag of the form `v1.2.3-rc` or `v1.2.3-rc.N` (N ≥ 2).
The first RC for a given semver gets no number; subsequent ones get `.2`, `.3`, etc.

```
v1.2.3-rc       ← first RC for 1.2.3
v1.2.3-rc.2     ← second
v1.2.3-rc.3     ← third
```

### In-flight RC
An RC tag whose base semver is **strictly greater than** the latest prod tag.
Example: latest prod is `v1.2.2`, `v1.3.0-rc.2` exists → `v1.3.0-rc.2` is in-flight.

If the latest prod tag is `v1.3.0`, then `v1.3.0-rc.2` is **not** in-flight — the prod release
already supersedes it.

---

## Commands

### `git release stage`

Creates or increments a release candidate.

**Unsuffixed (patch scope by default, but "continue what's there"):**

```sh
git release stage
```

- If an in-flight RC exists at **any** semver level → increment its counter.
  This is intentional: `stage` without a scope means "push another candidate for whatever
  I'm currently staging", not "start fresh".
- If no in-flight RC exists → bump the patch of the latest prod tag and create a new `-rc`.

Examples with latest prod `v1.2.2`:

| In-flight RC | Result |
|---|---|
| none | `v1.2.3-rc` |
| `v1.2.3-rc` | `v1.2.3-rc.2` |
| `v1.2.3-rc.4` | `v1.2.3-rc.5` |
| `v1.3.0-rc` | `v1.3.0-rc.2` |
| `v2.0.0-rc.3` | `v2.0.0-rc.4` |

---

**With explicit scope:**

```sh
git release stage minor
git release stage major
```

Calculates a **target semver** by applying the requested bump to the latest prod tag:

- `stage minor`: target = latest prod with minor incremented, patch reset to 0
- `stage major`: target = latest prod with major incremented, minor + patch reset to 0

Then checks for an in-flight RC and applies the following rules:

| Relationship of in-flight RC to target | Action |
|---|---|
| No in-flight RC | Create `<target>-rc` |
| In-flight RC base semver **< target** | Create `<target>-rc` (old RC tags are left; they are now stale) |
| In-flight RC base semver **== target** | Increment RC counter on in-flight tag |
| In-flight RC base semver **> target** | **Error** — cannot stage a lower scope when a higher one is already in flight. Clean up the RC tags manually before proceeding. |

Examples with latest prod `v1.2.2`:

| Command | In-flight RC | Target | Result |
|---|---|---|---|
| `stage minor` | none | `v1.3.0` | `v1.3.0-rc` |
| `stage minor` | `v1.2.3-rc` | `v1.3.0` | `v1.3.0-rc` (stale: `v1.2.3-rc`) |
| `stage minor` | `v1.3.0-rc.1` | `v1.3.0` | `v1.3.0-rc.2` |
| `stage minor` | `v2.0.0-rc` | `v1.3.0` | **Error** |
| `stage major` | none | `v2.0.0` | `v2.0.0-rc` |
| `stage major` | `v1.3.0-rc` | `v2.0.0` | `v2.0.0-rc` (stale: `v1.3.0-rc`) |
| `stage major` | `v2.0.0-rc.2` | `v2.0.0` | `v2.0.0-rc.3` |

---

### `git release`

Promotes or creates a production release.

**Unsuffixed ("continue what's there"):**

```sh
git release
```

- If an in-flight RC exists → promote it: check out the RC tag's commit, create a new prod
  tag at the same semver without the `-rc*` suffix, push both refs.
  Example: `v1.2.3-rc.4` → tag the same commit as `v1.2.3`.
- If no in-flight RC exists → bump the patch of the latest prod tag and tag + push immediately
  (no RC step).

---

**With explicit scope:**

```sh
git release minor
git release major
```

- If **no** in-flight RC exists → bump minor/major of the latest prod tag, tag + push.
- If an in-flight RC **exists** → prompt for confirmation:

  ```
  Warning: v1.3.0-rc.2 is in flight.
  Releasing v2.0.0 directly will bypass it.
  Type the version to confirm, or press Enter to cancel: _
  ```

  If the user types the exact version string, proceed. Otherwise abort.

---

### `git release status`

Prints a summary of the current release state:

```
Latest prod:    v1.2.2
In-flight RC:   v1.3.0-rc.2  (commit abc1234, 3 days ago)
Next stage:     v1.3.0-rc.3  (git release stage)
Next release:   v1.3.0       (git release)
```

---

### `git release ls`

Lists all release-related tags in descending semver order, annotated:

```
v1.3.0-rc.2   abc1234  2026-03-01  [in-flight RC]
v1.3.0-rc     def5678  2026-02-28  [superseded RC]
v1.2.2        789abcd  2026-02-20  [latest prod]
v1.2.1        ...
```

---

## Tag format

By default tags are prefixed with `v`. This is configurable (see Configuration).
All version comparisons use strict semver ordering (semver.org).

RC suffix format:
- First candidate: `-rc` (not `-rc.1`)
- Subsequent: `-rc.2`, `-rc.3`, …

---

## Configuration

Per-repo configuration via `git config` (`.git/config` or `--global`):

```sh
# Tag prefix (default: "v")
git config release.tagPrefix "v"

# Remote to push tags to (default: "origin")
git config release.remote "origin"

# Branch that releases must be made from (default: unset = any branch)
git config release.releaseBranch "main"

# Whether to require a clean working tree (default: true)
git config release.requireClean true

# GPG-sign tags (default: false)
git config release.signTags false

# Push the release branch (fast-forward only) before pushing the tag, so the
# remote branch can never lag its own tag (default: true)
git config release.pushBranch true
```

---

## Behaviour guarantees

- **Always fetches tags from remote before computing** the next version. Never trusts local
  tag state alone.
- **Never force-pushes.** If a tag already exists remotely the command errors with a clear
  message.
- **Dry-run mode** available on all mutating commands via `--dry-run` / `-n`: prints what
  would happen without creating or pushing any tags.
- **Non-interactive mode** via `--yes` / `-y`: answers all confirmation prompts affirmatively.
  Useful in CI.
- **Exit codes**: 0 success, 1 user abort, 2 error (wrong state, missing deps, etc.)

---

## Full example walkthrough

```sh
# Start from v1.2.2 in production

git release status
# Latest prod:  v1.2.2
# In-flight RC: none
# Next stage:   v1.2.3-rc
# Next release: v1.2.3

git release stage
# Created and pushed: v1.2.3-rc → abc1234

git release stage
# Created and pushed: v1.2.3-rc.2 → def5678

# Decide this should be a minor release instead
git release stage minor
# Created and pushed: v1.3.0-rc → def5678
# Note: v1.2.3-rc and v1.2.3-rc.2 are now stale (not deleted)

git release stage
# In-flight RC is v1.3.0-rc — incrementing
# Created and pushed: v1.3.0-rc.2 → ghi9012

# Ready to ship
git release
# Promoting v1.3.0-rc.2 → v1.3.0
# Created and pushed: v1.3.0 → ghi9012

git release status
# Latest prod:  v1.3.0
# In-flight RC: none
# Next stage:   v1.3.1-rc
# Next release: v1.3.1
```

---

## Implementation notes (for contributors)

- Written as a single self-contained shell script or compiled binary — both approaches are
  acceptable as long as the binary is named `git-release`.
- Semver parsing must handle the full spec including pre-release comparison rules. A dedicated
  semver library is preferred over hand-rolled parsing.
- Tag listing: `git ls-remote --tags <remote>` for authoritative remote state; fall back to
  local `git tag -l` only for `--dry-run` or `--offline` mode.
- All output to stdout; errors and warnings to stderr.
- Confirmation prompts must read from `/dev/tty` directly (not stdin) so the command is safe
  to use in pipelines while still being interactive when needed.
