# Grant on a specific table
terraform import scylladb_grant.example "admin|SELECT|TABLE|cycling|races"

# Grant on a keyspace
terraform import scylladb_grant.example "admin|ALTER|KEYSPACE|cycling|"

# Grant on all keyspaces
terraform import scylladb_grant.example "admin|MODIFY|ALL KEYSPACES||"
