# Invopop CLI

Invopop Command Line Interface - manage your Invopop account, build GoBL documents, and run workflows from the terminal.

## Introduction

The objective of the Invopop CLI is to demonstrate how the GoBL library and Invopop services can be used together to create digital invoices.

The application takes the provided yaml document and attempts to fill out all the missing details in order to output an official GoBL envelope. These can then be forwarded to Invopop for processing using a pre-defined workflow.

## How it Works

On one side the CLI expects a configuration file with the base information of the supplier (your company) and digital signature private key details. These details are used on all invoices being generated.

On the other, individual source data files in YAML define the basic input to be converted into complete GoBL JSON. The GoBL library automatically calculates taxes and totals, alongside validating that the contents look correct according to the region.

## Build & Install

Just execute the regular go build command:

```go
go build ./cmd/invopop
```

If you'd like to install the project in your global go bin directory (recommended), a simple `go install` will work:

```bash
go install github.com/invopop/cli/cmd/invopop
```

This will produce a binary named `invopop`.

## Configuration

When executing `invopop`, we check for configurations in two locations:

- `./config/config.yaml`, i.e. the configuration inside the current path.
- `~/.invopop/config.yaml`, a shared path in your user's home.

You can use the [sample](https://github.com/invopop/cli/blob/main/config/config.yaml) to get started, and replace the relevant fields with the data for your environment. By default, `invopop` comes with configuration details for a testing environment.

Most likely, you'll want to replace the `key` configuration and `api` endpoints.

### Keys

Generating a new ECDSA keypair is very straightforward. Simply run:

```bash
invopop keys generate
```

```
Private Key:
{"use":"sig","kty":"EC","kid":"540d1414-ef88-4eb0-a7c9-16dca0e61252","crv":"P-256","alg":"ES256","x":"HT1cpo-joEGg2Hsd68Bcjv9gzWzX4uTlijjUUaMxNTw","y":"_1VS8OzbIklFz5i5heYP4GZ9_f3x0YIoNh7l2RiZnKo","d":"BkElJC9P-bdqGhq8jxvy-0eOZe5OAst6oO6KGEJHHDc"}
Public Key:
{"use":"sig","kty":"EC","kid":"540d1414-ef88-4eb0-a7c9-16dca0e61252","crv":"P-256","alg":"ES256","x":"HT1cpo-joEGg2Hsd68Bcjv9gzWzX4uTlijjUUaMxNTw","y":"_1VS8OzbIklFz5i5heYP4GZ9_f3x0YIoNh7l2RiZnKo"}
```

Copy the "Private Key" to your `config.yaml`.

### API Host & Token

There are two API hosts currently in use:

- `https://api.invopop.dev` - exclusively for local development.
- `https://api.invopop.com` - production usage.

To generate an API token for your account, you must first log into the [Invopop Console](https://console.invopop.com). In the "Settings" section, you can access "Tokens" and click "New Token" to follow the steps to generate a new code. We use JSON Web Tokens for authentication, if you're interested, you can check the contents at [jwt.io](https://jwt.io/).

## Running

Execute the binary with the `build` argument and the source YAML file:

```bash
invopop build ./data/2021-06-16-invoice.yaml
```

If all goes well, a new complete json file will be output into the same directory.
