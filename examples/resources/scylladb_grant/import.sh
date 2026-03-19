# Import a Grant resource for a specific table with SELECT permission
terraform import scylladb_grant.example "admin|SELECT|TABLE|cycling|races"

# Import a Grant resource for a keyspace with ALTER permission
terraform import scylladb_grant.example "admin|ALTER|KEYSPACE|cycling|"

# Import a Grant resource for all keyspaces with MODIFY permission
terraform import scylladb_grant.example "admin|MODIFY|ALL KEYSPACES||"
