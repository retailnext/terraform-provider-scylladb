// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"bytes"
	"log"
	"text/template"
)

const (
	deleteGrantTemplate = `REVOKE {{ .Privilege }} ON {{.ResourceType}} {{if .Keyspace }}"{{ .Keyspace}}"{{end}}{{if and .Keyspace .Identifier}}.{{end}}{{if .Identifier}}"{{.Identifier}}"{{end}} FROM "{{.RoleName}}"`
	createGrantTemplate = `GRANT {{ .Privilege }} ON {{.ResourceType}} {{if .Keyspace }}"{{ .Keyspace}}"{{end}}{{if and .Keyspace .Identifier}}.{{end}}{{if .Identifier}}"{{.Identifier}}"{{end}} TO "{{.RoleName}}"`
	readGrantTemplate   = `LIST {{ .Privilege }} ON {{.ResourceType}} {{if .Keyspace }}"{{ .Keyspace }}"{{end}}{{if and .Keyspace .Identifier}}.{{end}}{{if .Identifier}}"{{.Identifier}}"{{end}} OF "{{.RoleName}}"`
)

var (
	templateDelete, _ = template.New("deleteGrant").Parse(deleteGrantTemplate)
	templateCreate, _ = template.New("createGrant").Parse(createGrantTemplate)
	templateRead, _   = template.New("readGrant").Parse(readGrantTemplate)
)

type Grant struct {
	Privilege    string
	ResourceType string
	RoleName     string
	Keyspace     string
	Identifier   string
}

// Note that `list <prvilege>` in cassandra returns different columnns.
type Permission struct {
	Role       string `cql:"role"`
	Username   string `cql:"username"` // for compatibility with Cassandra
	Resource   string `cql:"resource"`
	Permission string `cql:"permission"`
}

func (c *Cluster) CreateGrant(grant Grant) error {
	var queryBuffer bytes.Buffer
	err := templateCreate.Execute(&queryBuffer, grant)
	if err != nil {
		return err
	}
	queryStr := queryBuffer.String()
	log.Printf("Executing CreateGrant query: %s", queryStr)

	return c.Session.Query(queryStr).Exec()
}

func (c *Cluster) DeleteGrant(grant Grant) error {
	var queryBuffer bytes.Buffer
	err := templateDelete.Execute(&queryBuffer, grant)
	if err != nil {
		return err
	}
	queryStr := queryBuffer.String()
	log.Printf("Executing DeleteGrant query: %s", queryStr)

	return c.Session.Query(queryStr).Exec()
}

func (c *Cluster) GetGrant(grant Grant) ([]Permission, bool, error) {
	var queryBuffer bytes.Buffer
	err := templateRead.Execute(&queryBuffer, grant)
	if err != nil {
		return nil, false, err
	}
	queryStr := queryBuffer.String()
	log.Printf("Executing ReadGrant query: %s", queryStr)

	iter := c.Session.Query(queryStr).Iter()
	defer iter.Close()

	var permissions []Permission
	var p Permission
	found := false

	for iter.Scan(&p.Role, &p.Username, &p.Resource, &p.Permission) {
		found = true
		permissions = append(permissions, p)
	}

	return permissions, found, nil
}

func (c *Cluster) UpdateGrant(fromGrant, toGrant Grant) error {
	// No direct update in CQL, so we delete and create again
	if err := c.DeleteGrant(fromGrant); err != nil {
		return err
	}
	return c.CreateGrant(toGrant)
}
