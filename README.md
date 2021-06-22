# InvoCLI

Invoicing Command Line Interface - creating invoices quickly and easier from the command line.

## Introduction

The objective of InvoCLI is to demonstrate how the GoBL library and Invopop services can be used together to create digital invoices.

The application takes the provided yaml document and attempts to fill out all the missing details in order to output an official GoBL envelope. These can then be forwarded to Invopop for conversion into a PDF.

## How it Works

On one side InvoCLI expects a configuration file with the base information of the supplier (your company) and digital signature private key details. These details are used on all invoices being generated.

On the other, individual source data files in YAML define the basic input to be converted into complete GoBL JSON. The GoBL library automatically calculates taxes and totals, alongside validating that the contents look correct according to the region.

## Building

Just execute the regular go build command:

```go
go build .
```

## Running

Execute the binary with the `build` argument and the source YAML file:

```bash
./invocli build ./data/2021-06-16-invoice.yaml
```

If all goes well, a new complete json file will be output into the same directory.
