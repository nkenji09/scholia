# product-memory (`pmem`)

[![test](https://github.com/nkenji09/product-memory/actions/workflows/test.yml/badge.svg)](https://github.com/nkenji09/product-memory/actions/workflows/test.yml)

**A foundation for accumulating product decisions and their reasoning (why), linked to implementation changes, so they can be evaluated later.**

`pmem` is a CLI tool that records the detailed behavior of components and flows not as free-form text, but as **combinations of a controlled vocabulary**.

Code and tests preserve *what* was built and *how*, but *why* a given design was chosen tends to evaporate.
When collaborating with AI or working on a long-lived codebase, this loss of "why" is especially painful.
Without a readable record of past decisions, review comments get patched ad hoc — contradicting earlier decisions — and the same arguments get relitigated over and over.

`pmem` does not replace tests or reviews.
It adds one layer on top — a context layer of decisions and specs — and turns it into rules that the next round of work (human or AI) can read and follow.
All records are stored as plain JSON inside the target repository and are versioned in git alongside the code.
The viewer for browsing them ships as a single binary, so no extra runtime or database is required.

## Concept

`pmem`'s design follows from these core principles (the reasoning behind each is expanded in the why document):

- **Store only atoms; derive structure.** The only thing stored is the transition — an atom. Specs, hierarchies, and groupings are all derived from tags and queries.
- **Classify along three axes.** Category (fixed), kind (declared per project), and tag (free-form, nestable, cross-cutting classification) — nothing more.
- **Let git be the database.** One record, one text file. History, diffs, and review all run on plain git — no dedicated database needed.
- **Decisions are append-only.** A decision is never deleted or edited; a correction is added as a new entry. Frozen judgments become the baseline against which future changes are evaluated.
- **Vocabulary and tags are orthogonal.** Vocabulary (vocab) composes behavior; tags classify it (tags classify; vocab composes).

The diagram below shows the relationship between the atoms that get stored and the derived views built from them.

```mermaid
flowchart LR
    act["action"] --> tx["transition (atom)"]
    cond["condition"] --> tx
    eff["effect"] --> tx
    tag["tag (cross-cutting classification)"] -. classifies .-> tx
    dec["decision (why / append-only)"] -. attaches to .-> tx
    dec -. attaches to .-> tag
    tx --> q["derived views: spec / tag hierarchy / rules<br/>(not stored — derived via query)"]
    tag --> q
```

Design decisions that dig into "why" for each principle are expanded in [Why product-memory](docs/why-pmem.ja.md) (Japanese).
To see what an actual record looks like, try `pmem spec` and `pmem view` from the quickstart below.

## Installation

If you have Go, `go install` works.

```sh
go install github.com/nkenji09/product-memory/cmd/pmem@latest
```

Prebuilt binaries (darwin/linux/windows × amd64/arm64) are available from GitHub Releases.
The viewer SPA is embedded into the binary via `//go:embed`, so a single `pmem` binary runs both the CLI and the viewer.

## Quickstart

This walks through the minimal flow: create `.pmem/`, add vocabulary, tags, and a transition one at a time, and record a decision.

```sh
# 1. Create .pmem/ in your project
pmem init

# 2. Add vocabulary (action / condition / effect)
pmem vocab add action    act.user.submit-login   --label "Submit login" --kind user
pmem vocab add condition cond.credentials-valid  --label "Credentials are valid"
pmem vocab add effect    eff.session.issue-token --label "Issue session token" --kind state --owner server

# 3. Add a cross-cutting classification tag
pmem tag create subject.auth --name "Authentication" --kind subject

# 4. Add a transition (atom): WHEN submit login GIVEN credentials valid THEN issue token
pmem tx add T-login-submit-valid \
  --action act.user.submit-login \
  --given  cond.credentials-valid \
  --then   eff.session.issue-token \
  --tags   subject.auth

# 5. Record a decision (why) — append-only
pmem decide --on transition:T-login-submit-valid \
  --why "Issue the token as an httpOnly cookie (XSS mitigation)" --ref "PR#42"

# 6. Check the records for self-contradiction
pmem lint

# 7. View the "spec" report grouped by subject tag (derived view)
pmem spec subject.auth
```

Step 7 renders a derived report like this:

```
# Authentication (subject.auth)

## T-login-submit-valid
WHEN Submit login GIVEN Credentials are valid THEN Issue session token
decisions:
  - Issue the token as an httpOnly cookie (XSS mitigation) (PR#42)
```

To browse and evaluate records in a browser, start the local viewer.

```sh
pmem view   # opens at http://127.0.0.1:4577
```

The viewer includes tag-hierarchy navigation, requirement traceability, and an evaluation drawer that checks uncommitted changes against past decisions.

## Screenshots

The viewer running against this repository's own `.pmem/` records (dogfooding).

| | |
|---|---|
| ![Tag tree](docs/images/tag-tree.png) Tag index tree, grouped by category (requirement / concern / component) | ![Spec card](docs/images/spec-card.png) A transition spec card — trigger, given, result, tags, and its decision |
| ![Tag decisions](docs/images/tag-decisions.png) A requirement tag's user story, related specs, and accumulated decisions | ![Home](docs/images/home.png) Home — requirement traceability and recent decisions at a glance |

## Records are written through the CLI

Don't edit files under `.pmem/` directly in a text editor.
`pmem` handles reads and writes consistently, enforcing normalization, invariant checks, and the append-only guarantee on decisions.
Writing by hand breaks these guarantees and undermines the reliability of the records.

## For AI agents

`pmem rules` surfaces the rules to follow, and `pmem decision list` surfaces past judgments, both in machine-readable form.
`pmem show vocab <id>` reverse-looks-up the transitions that reference a given vocabulary entry — the true impact set for a safe refactor.

Claude Code skills (`pmem` / `pmem-change` / `pmem-triage` / `pmem-config-setup`) are bundled under `agents/skills/`, and there are two ways to install them:

**A. As a Claude Code plugin (recommended for Claude Code users).** Add this repository as a plugin marketplace and install the `pmem` plugin. The skills are then namespaced as `/pmem:pmem`, `/pmem:pmem-change`, etc.

```
/plugin marketplace add nkenji09/product-memory
/plugin install pmem@product-memory
```

**B. Via the CLI (`pmem skills install`).** Unpacks the same skills into `.claude/skills/` from the embedded copy in the binary — no marketplace needed. Handy for CI, standalone environments, or when you install `pmem` via `go install` and want the skills materialized in-repo without going through the plugin flow.

```sh
pmem skills install            # into <cwd>/.claude/skills/ (default)
pmem skills install --user     # into ~/.claude/skills/
```

Both paths ship the **same single source** (`agents/skills/`); the plugin serves it via the marketplace, while `pmem skills install` serves it from the binary's `//go:embed` copy. The plugin version (`agents/.claude-plugin/plugin.json`) is tracked independently of the CLI release version.

## License

MIT License. See [LICENSE](LICENSE).
