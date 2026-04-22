CREATE KEYSPACE IF NOT EXISTS mykeyspace
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE mykeyspace;

CREATE TABLE IF NOT EXISTS mytable (
    id uuid PRIMARY KEY,
    name text,
    created_at timestamp
);
