# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-19

### Added

#### Provider

- `host` attribute (or `SCYLLADB_HOST` env var) to configure the ScyllaDB endpoint
- `system_auth_keyspace` attribute to override the default `system_auth` keyspace
- `ca_cert_file` and `skip_host_verification` attributes for TLS configuration
- `auth_login_userpass` block for username/password authentication (`SCYLLADB_PASSWORD` env var supported)
- `auth_tls` block for mutual TLS (mTLS) authentication (`SCYLLADB_CLIENT_CERT` / `SCYLLADB_CLIENT_KEY` env vars supported)
- HTTP proxy support with ad-hoc DNS resolution for connections through a proxy

#### Resources

- `scylladb_role` — manage ScyllaDB roles including login, superuser, and `member_of` membership
- `scylladb_grant` — manage grants of permissions on keyspaces/tables to roles, with drift detection

#### Data Sources

- `scylladb_role` — read an existing ScyllaDB role
