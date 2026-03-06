# Authentication with username and password
provider "scylladb" {
  host = "localhost:9042"
  auth_login_userpass {
    username = "cassandra"
    password = "cassandra"
  }
}

# Authentication with mTLS and a CA certificate
provider "scylladb" {
  host                   = "scylladb.example.com:9142"
  ca_cert_file           = "/etc/ssl/scylladb/ca.crt"
  skip_host_verification = false
  auth_login_userpass {
    username = "myuser"
    password = "mypassword"
  }
  auth_tls {
    cert_file = "/etc/ssl/scylladb/client.crt"
    key_file  = "/etc/ssl/scylladb/client.key"
  }
}
