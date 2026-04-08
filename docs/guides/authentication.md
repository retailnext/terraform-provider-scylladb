---
page_title: "Authentication - ScyllaDB Provider"
description: |-
  Configure authentication for the ScyllaDB provider using username/password, TLS, or environment variables.
---

# Authentication

The ScyllaDB provider supports username/password authentication and mutual TLS (mTLS). Credentials can be supplied via provider attributes or environment variables.

## Username and Password

The simplest authentication method uses ScyllaDB's built-in role-based credentials:

```terraform
provider "scylladb" {
  host = "localhost:9042"
  auth_login_userpass {
    username = "cassandra"
    password = "cassandra"
  }
}
```

## Environment Variables

Sensitive values can be kept out of Terraform configuration files using environment variables. The provider reads these before applying provider attributes:

| Variable | Description |
|---|---|
| `SCYLLADB_HOST` | Hostname or IP address of the ScyllaDB instance (overridden by `host`) |
| `SCYLLADB_USERNAME` | Username for username/password authentication (overridden by `auth_login_userpass.username`) |
| `SCYLLADB_PASSWORD` | Password for username/password authentication (overridden by `auth_login_userpass.password`) |
| `SCYLLADB_CA_CERT` | PEM-encoded CA certificate (alternative to `ca_cert_file`) |
| `SCYLLADB_CLIENT_CERT` | PEM-encoded client certificate (alternative to `auth_tls.cert_file`) |
| `SCYLLADB_CLIENT_KEY` | PEM-encoded client private key (alternative to `auth_tls.key_file`) |

With environment variables set, the provider block can be minimal:

```terraform
# SCYLLADB_HOST=localhost:9042
# SCYLLADB_PASSWORD=cassandra
provider "scylladb" {
  auth_login_userpass {
    username = "cassandra"
  }
}
```

## TLS with a CA Certificate

To verify the server's identity, provide a CA certificate:

```terraform
provider "scylladb" {
  host         = "scylladb.example.com:9142"
  ca_cert_file = "/etc/ssl/scylladb/ca.crt"
  auth_login_userpass {
    username = "myuser"
    password = "mypassword"
  }
}
```

Set `skip_host_verification = true` only in development environments where the server certificate cannot be verified:

```terraform
provider "scylladb" {
  host                   = "localhost:9142"
  ca_cert_file           = "/tmp/ca.crt"
  skip_host_verification = true
  auth_login_userpass {
    username = "cassandra"
    password = "cassandra"
  }
}
```

## Mutual TLS (mTLS)

For clusters that require client certificate authentication, provide both a CA certificate and client key pair:

```terraform
provider "scylladb" {
  host         = "scylladb.example.com:9142"
  ca_cert_file = "/etc/ssl/scylladb/ca.crt"
  auth_tls {
    cert_file = "/etc/ssl/scylladb/client.crt"
    key_file  = "/etc/ssl/scylladb/client.key"
  }
}
```

Alternatively, supply the PEM content via environment variables instead of file paths:

```shell
export SCYLLADB_CA_CERT="$(cat /etc/ssl/scylladb/ca.crt)"
export SCYLLADB_CLIENT_CERT="$(cat /etc/ssl/scylladb/client.crt)"
export SCYLLADB_CLIENT_KEY="$(cat /etc/ssl/scylladb/client.key)"
```

```terraform
provider "scylladb" {
  host = "scylladb.example.com:9142"
  auth_login_userpass {
    username = "myuser"
    password = "mypassword"
  }
}
```
