resource "scylladb_role" "admin" {
  role      = "admin"
  can_login = false
}

resource "scylladb_role" "readonly" {
  role      = "readonly"
  can_login = true
}

# Authoritatively manages all grants on cycling.cyclist_name.
# Any grants on that table not listed here will be revoked on apply.
resource "scylladb_table_grants" "cycling_cyclist_name" {
  keyspace = "cycling"
  table    = "cyclist_name"

  grant {
    role       = scylladb_role.admin.role
    privileges = ["ALTER", "MODIFY", "SELECT"]
  }

  grant {
    role       = scylladb_role.readonly.role
    privileges = ["SELECT"]
  }
}

provider "scylladb" {
  host                 = "localhost:9042"
  system_auth_keyspace = "system"
  auth_login_userpass {
    username = "cassandra"
    password = "cassandra"
  }
}
