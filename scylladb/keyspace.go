// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package scylladb

import (
	"fmt"
	"log"
)

type Keyspace struct {
	Name              string
	ReplicationClass  string
	ReplicationFactor int
	DurableWrites     bool
}

func (c *Cluster) CreateKeyspace(ks Keyspace) error {
	query := fmt.Sprintf(`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': '%s', 'replication_factor': %d} AND durable_writes = %v`,
		ks.Name,
		ks.ReplicationClass,
		ks.ReplicationFactor,
		ks.DurableWrites,
	)
	log.Printf("Executing CreateKeyspace query: %s", query)
	return c.Session.Query(query).Exec()
}

func (c *Cluster) DeleteKeyspace(ks Keyspace) error {
	query := fmt.Sprintf(`DROP KEYSPACE IF EXISTS %s`, ks.Name)
	return c.Session.Query(query).Exec()
}
