resource "scylladb_role" "admin" {
  role         = "admin"
  can_login    = false
  is_superuser = false
}

resource "scylladb_grant" "admin_alter_cycling" {
  role_name     = scylladb_role.admin.role
  privilege     = "ALTER"
  resource_type = "KEYSPACE"
  keyspace      = "cycling"
}
