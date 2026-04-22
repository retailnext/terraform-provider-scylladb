// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"fmt"
	"log"
	"slices"
	"strings"
)

// ParsedIdentifier represents a parsed resource identifier — either a keyspace or a table.
type ParsedIdentifier struct {
	ResourceType string // "KEYSPACE" or "TABLE"
	Keyspace     string
	Table        string // empty for KEYSPACE type
	Original     string // original string as supplied by the user (e.g. "cycling" or "cycling.cyclist_name")
}

// AuthoritativeBinding represents a set of privileges granted to a role.
type AuthoritativeBinding struct {
	Privileges []string
	Role       string
}

// const ValidPrivileges lists the privileges that are allowed in an authoritative binding for keyspace and table resources.
var ValidPrivileges = []string{
	"ALTER",
	"AUTHORIZE",
	"CREATE",
	"DROP",
	"MODIFY",
	"SELECT",
}

// Validate returns an error if any privilege in the binding is not one of ValidPrivileges.
func (b AuthoritativeBinding) Validate() error {
	for _, priv := range b.Privileges {
		if !slices.Contains(ValidPrivileges, strings.ToUpper(priv)) {
			return fmt.Errorf("privilege %q is not allowed in an authoritative binding", priv)
		}
	}
	return nil
}

// ParseIdentifier returns if the given string represents a keyspace or a table.
// A string containing "." is treated as "keyspace.table"; otherwise it is a keyspace.
func ParseIdentifier(identifier string) ParsedIdentifier {
	parts := strings.SplitN(identifier, ".", 2)
	if len(parts) == 2 {
		return ParsedIdentifier{
			ResourceType: "TABLE",
			Keyspace:     parts[0],
			Table:        parts[1],
			Original:     identifier,
		}
	}
	return ParsedIdentifier{
		ResourceType: "KEYSPACE",
		Keyspace:     identifier,
		Original:     identifier,
	}
}

// ApplyAuthoritativeGrant reconciles the grants on identifier so they exactly match bindings.
// Privileges present in the database but absent from bindings are revoked; privileges in
// bindings but absent from the database are granted.
func (c *Cluster) ApplyAuthoritativeGrant(identifier ParsedIdentifier, bindings []AuthoritativeBinding) error {
	// Validate bindings first
	for _, b := range bindings {
		if err := b.Validate(); err != nil {
			return err
		}
	}

	// Get current permissions using role_permissions table
	currentPermsMap, err := c.GetAllRolePermissionsPerId(identifier)
	if err != nil {
		return err
	}

	// Build desired permissions lookup
	desired := make(map[string]map[string]bool)
	for _, b := range bindings {
		role := b.Role
		if desired[role] == nil {
			desired[role] = make(map[string]bool)
		}
		for _, priv := range b.Privileges {
			desired[role][strings.ToUpper(priv)] = true
		}
	}

	// Revoke grants not present in desired
	for role, privs := range currentPermsMap {
		for _, priv := range privs {
			if !desired[role][strings.ToUpper(priv)] {
				if err := c.DeleteGrant(Grant{
					RoleName:     role,
					Privilege:    priv,
					ResourceType: identifier.ResourceType,
					Keyspace:     identifier.Keyspace,
					Identifier:   identifier.Table,
				}); err != nil {
					return err
				}
			}
		}
	}

	// Build current permissions lookup
	current := make(map[string]map[string]bool)
	for role, privs := range currentPermsMap {
		current[role] = make(map[string]bool)
		for _, p := range privs {
			current[role][strings.ToUpper(p)] = true
		}
	}

	// Grant privileges that are missing
	for role, privs := range desired {
		for priv := range privs {
			if !current[role][priv] {
				if err := c.CreateGrant(Grant{
					RoleName:     role,
					Privilege:    priv,
					ResourceType: identifier.ResourceType,
					Keyspace:     identifier.Keyspace,
					Identifier:   identifier.Table,
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// RevokeAllGrantsOnIdentifier revokes every grant held by every role on identifier.
// Used during resource destroy to leave no residual permissions behind.
func (c *Cluster) RevokeAllGrantsOnIdentifier(id ParsedIdentifier) error {
	current, err := c.GetAllRolePermissionsPerId(id)
	if err != nil {
		return err
	}
	for role, privs := range current {
		for _, priv := range privs {
			if err := c.DeleteGrant(Grant{
				RoleName:     role,
				Privilege:    priv,
				ResourceType: id.ResourceType,
				Keyspace:     id.Keyspace,
				Identifier:   id.Table,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetEffectivePermissions returns a sorted flat list of "identifier:role:PRIVILEGE" strings
// for all current grants on identifier. Used as the computed state stored in Terraform so
// that plan/apply can detect out-of-band permission changes.
func (c *Cluster) GetEffectivePermissions(id ParsedIdentifier) ([]string, error) {
	perms := []string{}
	current, err := c.GetAllRolePermissionsPerId(id)
	if err != nil {
		return nil, err
	}
	for role, privs := range current {
		for _, priv := range privs {
			perms = append(perms, id.Original+":"+role+":"+strings.ToUpper(priv))
		}
	}
	slices.Sort(perms)
	return perms, nil
}

// GetAllRolePermissionsPerId reads the role_permissions system table to return the current
// permissions for every role on identifier, keyed by role name. This is the source of truth
// used by ApplyAuthoritativeGrant to compute revoke/grant diffs.
func (c *Cluster) GetAllRolePermissionsPerId(id ParsedIdentifier) (permissionMap map[string][]string, err error) {
	resourceName := getResourceName(Grant{
		ResourceType: id.ResourceType,
		Keyspace:     id.Keyspace,
		Identifier:   id.Table,
	})
	if resourceName == "" {
		return nil, fmt.Errorf("no valid resource name is found for identifier: %v", id)
	}

	queryStr := fmt.Sprintf("SELECT role, permissions FROM %s.role_permissions WHERE resource = ?", c.SystemAuthKeyspaceName)
	log.Printf("Executing ReadGrant query: %s", queryStr)

	iter := c.Session.Query(queryStr, resourceName).Iter()

	permissionMap = make(map[string][]string)
	var role string
	var permissions []string
	for iter.Scan(&role, &permissions) {
		permissionMap[role] = permissions
	}

	if err = iter.Close(); err != nil {
		return nil, err
	}
	return permissionMap, nil
}
