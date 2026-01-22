# Terraform Provider Scylladb

This provider plugin allows to configure the access control for Scylladb through Terraform,
including roles, keyspaces, and grants. It contains the following

- `internal/provider/`: A resource and a data source
- `examples/`: examples
- `docs/`: generated documentation,
- `scylladb/`: client for scylladb and abstracted methods to update ACL in scylladb
- Miscellaneous meta files.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.4
- [Go](https://golang.org/doc/install) >= 1.24

## Building The Provider

1. Clone the repository to `$GOPATH/src/github.com/hashicorp/terraform-provider-scylladb`
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources and scylladb containers, and it would run slower than typical unittests.

```shell
make testacc
```
