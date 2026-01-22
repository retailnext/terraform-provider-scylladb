# Manage admin role
resource "scylladb_role" "admin" {
  role         = "admin"
  can_login    = false
  is_superuser = false
}
