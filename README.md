# Terraform Provider Scylladb

This provider plugin allows to configure the access control for Scylladb through Terraform,
including roles, keyspaces, and grants. It contains the following

- `internal/provider/`: A resource and a data source
- `examples/`: examples
- `docs/`: generated documentation,
- `scylladb/`: client for scylladb and abstracted methods to update ACL in scylladb

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

### Using the local provider
The following is a general guidance on how to use the local provider you are developing in terraform code before it is published.

1. Using `dev_overrides` path

    Follow the official direction, [here](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider-configure).
    This allows to use the provider by running `go install .`. Your `~/.terraformrc` would look like

    ```
    provider_installation {

    dev_overrides {
        "registry.terraform.io/retailnext/scylladb" = "/Users/myusername/go/bin"
    }

    # For all other providers, install them directly from their origin provider
    # registries as normal. If you omit this, Terraform will _only_ use
    # the dev_overrides block, and so no other providers will be available.
    direct {}
    }
    ```

    Please note that you cannot run `terraform init`. That means, if you have other non-local providers in your terraform code,
    you will not be able to run `terraform init`. In this case, use the following "release" binary method.

    If you are using `tofu`, update `~/.tofurc` and use `registry.opentofu.org` as the provider registry.

2. Using the local "release" binary

    By manually doing the steps which `terraform init` would have, you can use the local code. The following example shows the steps in a linux environment with amd64 processor.
    ```
    CGO_ENABLED=0 go build -trimpath -o terraform-provider-scylladb_v1.0.0 -ldflags "-s -w -X main.version=1.0.0" .
    mkdir -p ~/.terraform.d/plugins/local.providers/local/scylladb/1.0.0/linux_amd64
    mv terraform-provider-scylladb_v1.0.0 ~/.terraform.d/plugins/local.providers/local/scylladb/1.0.0/linux_amd64

    cat <<EOF > $HOME/.terraformrc
    provider_installation {
        filesystem_mirror {
            path = "/home/runner/.terraform.d/plugins"
            include = ["local.providers/*/*"]
        }
        direct {
            exclude = ["local.providers/*/*"]
        }
    }
    EOF
    ```

    If you are using `tofu`, use `$HOME/.tofurc` instead of `$HOME/.terraformrc` in the example.
