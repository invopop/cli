# Invopop CLI

Command-line interface for the [Invopop](https://invopop.com) platform. Build and validate [GOBL](https://gobl.org) documents, manage silo entries and workflows, run transform jobs, and generate sequence numbers — all from your terminal.

## Installation

### Pre-built binaries

Download the latest release for your platform from the [releases page](https://github.com/invopop/cli/releases).

### From source

```bash
go install github.com/invopop/cli/cmd/invopop@latest
```

This produces a binary called `invopop` in your `$GOBIN`.

## Authentication

### Sign in with your account (recommended)

```bash
invopop auth login
```

This opens your browser to sign in to Invopop and asks you to paste back the
code shown there, which works over SSH or from a container too (use
`--no-browser` to just print the link). It then asks which workspace to use
by default, or pass `--workspace <id-or-slug>`. From then on, commands use a
short-lived token that is restricted to that workspace and carries no more
than your own permissions there. The CLI renews it on its own, so no API key
is needed, and an agent driving the CLI can reach only what you can.

Sign-ins are stored per API host in your config directory (e.g.
`~/.config/invopop/credentials.json`), readable only by you, together with
the tokens for each workspace you have used.

```bash
invopop auth status                    # who you are and which workspace is in use
invopop auth workspace                 # choose another default workspace
invopop auth workspace my-shop         # switch the default directly
invopop auth workspace sandbox/my-shop # or to its sandbox
invopop auth token                     # print a workspace token for curl etc.
invopop auth logout
```

#### Working in several workspaces

The workspace chosen above is the default for every terminal. To act in
another one without changing it, name it with `--workspace` (`-w`) for a
single command, or set `INVOPOP_WORKSPACE` for a whole terminal or project
directory (e.g. with [direnv](https://direnv.net)):

```bash
invopop access workspaces              # every workspace you belong to, with its ref
invopop -w sandbox/my-shop silo entries list
export INVOPOP_WORKSPACE=my-other-shop
```

A workspace is named by its ID or its slug. A bare slug means a live
workspace; prefix it with `sandbox/` for a sandbox one, since the two can
share a slug. Each workspace keeps its own token, so moving between them is
quick. `invopop auth status` shows which workspace a command will use and
what chose it.

### API keys

Alternatively, generate a token from the [Invopop Console](https://console.invopop.com)
under **Settings → Tokens** and pass it via the `INVOPOP_TOKEN` environment
variable or the `--token` / `-t` flag. An explicit token always takes
precedence over a stored sign-in. A token already belongs to one workspace,
so it cannot be combined with `--workspace` or `INVOPOP_WORKSPACE`.

```bash
export INVOPOP_TOKEN=eyJhbGciOi...
invopop ping
```

The API host defaults to `https://api.invopop.com`. Set `--host` or the `INVOPOP_HOST`
environment variable for another environment, e.g. `https://api.invopop.dev` for local
development; tokens and sign-ins only work on the environment that issued them.

## Commands

| Command | Description |
|---|---|
| `invopop auth` | Sign in with your account and choose a default workspace. |
| `invopop ping` | Test API connectivity. |
| `invopop access` | Show the workspace in use, or list all those you belong to. |
| `invopop silo` | Manage silo entries, files, and GOBL documents. |
| `invopop transform` | Manage transform workflows and jobs. |
| `invopop sequence` | Manage sequence series and entries for invoice numbering. |
| `invopop tui` | Browse entries, workflows and jobs in an interactive terminal UI. |
| `invopop skill` | Install the Invopop skill for [Claude Code](https://claude.com/claude-code). |

Run any command with `--help` for full flag and subcommand details. Add
`-w <id-or-slug>` to any command to run it in a workspace other than the
default.

## Examples

Build and validate a GOBL invoice locally:

```bash
invopop silo build --file invoice.yaml
```

Sign a GOBL document with the workspace's key:

```bash
invopop silo sign --file invoice.yaml
```

Create a silo entry from a local document:

```bash
invopop silo entries create --file invoice.yaml
```

Run a transform job against an existing entry and wait for completion:

```bash
invopop transform jobs create \
    --workflow-id 01h2x5... \
    --silo-entry-id 01h2x6... \
    --wait 30
```

Commands that take a document accept YAML or JSON on `--file`/`-f`, or on stdin. Results are printed to stdout as JSON, easy to pipe into `jq` or another command; `silo files download` writes the raw file instead.

## Terminal UI

`invopop tui` opens an interactive view of your silo entries, workflows and
jobs. It uses the same credentials as every other command and opens in the
same workspace, so `invopop tui -w sandbox/my-shop` starts in that sandbox.
Press `w` to switch workspace from inside it: `enter` switches for the
session only, and `ctrl+d` also makes the choice your default. Switching
needs a sign-in, since an API token belongs to a single workspace.

## Claude Code integration

The CLI ships with a [Claude Code](https://claude.com/claude-code) skill that lets an AI agent drive your Invopop workspace through this binary. Install it with:

```bash
invopop skill install
```

Then invoke `/invopop` inside Claude Code.

## Documentation

- [Invopop docs](https://docs.invopop.com) — platform concepts, workflows, apps, and API reference.
- [GOBL docs](https://docs.gobl.org) — document schemas, tax regimes, and addons.
