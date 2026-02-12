// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"bytes"
	"fmt"
	"log"
	"strings"
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

func (c *Cluster) GetPermissionStrs(grant Grant) (permissions []string, err error) {
	resourceName := getResourceName(grant)
	if resourceName == "" {
		return
	}

	// the query should return only 1 record even without LIMIT 1.
	queryStr := fmt.Sprintf("SELECT permissions FROM %s.role_permissions WHERE role = ? AND resource = ? LIMIT 1", c.SystemAuthKeyspaceName)
	log.Printf("Executing ReadGrant query: %s", queryStr)

	err = c.Session.Query(queryStr, grant.RoleName, resourceName).Scan(&permissions)
	return
}

func getResourceName(grant Grant) string {
	switch strings.ToUpper(grant.ResourceType) {
	case "ALL KEYSPACES":
		return "data"
	case "KEYSPACE":
		return fmt.Sprintf("data/%s", grant.Keyspace)
	case "TABLE":
		return fmt.Sprintf("data/%s/%s", grant.Keyspace, grant.Identifier)
	case "ALL ROLES":
		return "roles"
	case "ROLE":
		return fmt.Sprintf("roles/%s", grant.Keyspace)
	default:
		return ""
	}
}

// func (g Grant) GetExpandedPermissions() []string {
// 	origPerm := strings.ToUpper(g.Privilege)
// 	if origPerm != "ALL PERMISSIONS" {
// 		return []string{origPerm}
// 	}
// 	switch strings.ToUpper(g.ResourceType) {
// 	case "ALL KEYSPACES", "KEYSPACE":
// 		return []string{"ALTER", "AUTHORIZE", "CREATE", "DROP", "MODIFY", "SELECT"}
// 	case "TABLE":
// 		return []string{"ALTER", "AUTHORIZE", "DROP", "MODIFY", "SELECT"}
// 	case "ALL ROLES", "ROLE":
// 		return []string{"ALTER", "AUTHORIZE", "DROP"}
// 	default:
// 		return []string{origPerm}
// 	}
// }
