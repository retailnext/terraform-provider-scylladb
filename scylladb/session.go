// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package scylladb

import (
	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

type Cluster struct {
	Cluster                *gocql.ClusterConfig
	SystemAuthKeyspaceName string
	Session                *gocql.Session
}

func NewClusterConfig(hosts []string) Cluster {
	cluster := gocql.NewCluster(hosts...)
	cluster.DisableInitialHostLookup = true
	return Cluster{
		Cluster:                cluster,
		SystemAuthKeyspaceName: "system_auth",
	}
}

func (c *Cluster) CreateSession() error {
	session, err := c.Cluster.CreateSession()
	if err != nil {
		return err
	}
	c.Session = session
	return nil
}

func (c *Cluster) SetUserPasswordAuth(username, password string) {
	c.Cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: username,
		Password: password,
	}
}

func (c *Cluster) SetSystemAuthKeyspace(name string) {
	c.SystemAuthKeyspaceName = name
}
