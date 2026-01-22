# authentication with username and password
provider "scylladb" {
  host = "localhost:9042"
  auth_login_userpass {
    username = "cassandra"
    password = "cassandra"
  }
}
