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
`--no-browser` to just print the link). It then asks which workspace to work
in, or pass `--workspace <id-or-slug>`. From then on, commands use a
short-lived token that is restricted to that workspace and carries no more
than your own permissions there. The CLI renews it on its own, so no API key
is needed, and an agent driving the CLI can reach only what you can.

```bash
invopop auth status              # who you are and which workspace is in use
invopop auth workspace           # choose another workspace
invopop auth workspace my-shop   # switch to it directly
invopop auth workspace --list    # list them as JSON
invopop auth token               # print a workspace token for curl etc.
invopop auth logout
```

Sign-ins are stored per API host in your config directory (e.g.
`~/.config/invopop/credentials.json`), readable only by you.

### API keys

Alternatively, generate a token from the [Invopop Console](https://console.invopop.com)
under **Settings → Tokens** and pass it via the `INVOPOP_TOKEN` environment
variable or the `--token` / `-t` flag. An explicit token always takes
precedence over a stored sign-in.

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
| `invopop auth` | Sign in with your account and choose a workspace. |
| `invopop ping` | Test API connectivity. |
| `invopop workspace` | Show details of the current workspace. |
| `invopop silo` | Manage silo entries, files, and GOBL documents. |
| `invopop transform` | Manage transform workflows and jobs. |
| `invopop sequence` | Manage sequence series and entries for invoice numbering. |
| `invopop tui` | Launch the interactive terminal UI for browsing entries, workflows, and jobs. |
| `invopop skill` | Install the Invopop skill for [Claude Code](https://claude.com/claude-code). |

Run any command with `--help` for full flag and subcommand details.

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

## Claude Code integration

The CLI ships with a [Claude Code](https://claude.com/claude-code) skill that lets an AI agent drive your Invopop workspace through this binary. Install it with:

```bash
invopop skill install
```

Then invoke `/invopop` inside Claude Code.

## Documentation

- [Invopop docs](https://docs.invopop.com) — platform concepts, workflows, apps, and API reference.
- [GOBL docs](https://docs.gobl.org) — document schemas, tax regimes, and addons.
