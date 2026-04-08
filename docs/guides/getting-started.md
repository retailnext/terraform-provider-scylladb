---
page_title: "Getting Started - ScyllaDB Provider"
description: |-
  Set up the ScyllaDB provider and manage your first role and grant.
---

# Getting Started

This guide walks through configuring the ScyllaDB provider and creating a role with granted permissions.

## Prerequisites

- A running ScyllaDB cluster with authentication enabled
- Terraform >= 1.4
- The user/role you connect with must have permissions to manage roles and grants (e.g. `cassandra` superuser)
  Note that the provider will try to perform as instructed, but if the user lacks permissions, Terraform will
  report errors during apply.

## Configure the Provider

Add the provider to your Terraform configuration and configure it to connect to your cluster:

```terraform
terraform {
  required_providers {
    scylladb = {
      source  = "retailnext/scylladb"
    }
  }
}

provider "scylladb" {
  host = "localhost:9042"
  auth_login_userpass {
    username = "cassandra"
    password = "cassandra"
  }
}
```

Run `terraform init` to install the provider.

## Create a Role

Roles are the primary unit of access control in ScyllaDB. A role can represent a user (with `can_login = true`) or a permission group:

```terraform
# A login user
resource "scylladb_role" "app_user" {
  role      = "app_user"
  can_login = true
}

# A permission group (not a login user)
resource "scylladb_role" "readonly" {
  role      = "readonly"
  can_login = false
}
```

## Grant Permissions

Use `scylladb_grant` to assign privileges to a role:

```terraform
# Allow the readonly group to SELECT from all keyspaces
resource "scylladb_grant" "readonly_select" {
  role_name     = scylladb_role.readonly.role
  privilege     = "SELECT"
  resource_type = "ALL KEYSPACES"
}

# Allow app_user to modify a specific keyspace
resource "scylladb_grant" "app_user_modify_cycling" {
  role_name     = scylladb_role.app_user.role
  privilege     = "MODIFY"
  resource_type = "KEYSPACE"
  keyspace      = "cycling"
}

# Allow app_user to SELECT from a specific table
resource "scylladb_grant" "app_user_select_races" {
  role_name     = scylladb_role.app_user.role
  privilege     = "SELECT"
  resource_type = "TABLE"
  keyspace      = "cycling"
  identifier    = "races"
}
```

## Read Role Information

Use the `scylladb_role` data source to read an existing role without managing it:

```terraform
data "scylladb_role" "cassandra" {
  id = "cassandra"
}

output "cassandra_is_superuser" {
  value = data.scylladb_role.cassandra.is_superuser
}
```
