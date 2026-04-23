resource "scylladb_role" "admin" {
  role      = "admin"
  can_login = false
}

resource "scylladb_role" "readonly" {
  role      = "readonly"
  can_login = false
}

# Authoritatively manages all grants on the cycling keyspace.
# Any grants on the keyspace not listed here will be revoked on apply.
resource "scylladb_keyspace_grants" "cycling" {
  keyspace = "cycling"

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
