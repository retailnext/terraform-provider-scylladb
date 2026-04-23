CREATE KEYSPACE IF NOT EXISTS cycling
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE cycling;

CREATE TABLE IF NOT EXISTS cyclist_name (
    id uuid PRIMARY KEY,
    name text,
    created_at timestamp
);
