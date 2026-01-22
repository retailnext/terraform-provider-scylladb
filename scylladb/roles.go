// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package scylladb

import (
	"fmt"
	"unicode"
)

type Role struct {
	Role        string
	CanLogin    bool
	IsSuperuser bool
	MemberOf    []string
}

func (c *Cluster) GetRole(roleName string) (Role, error) {
	var role Role
	query := fmt.Sprintf("SELECT role, can_login, is_superuser, member_of FROM %s.roles WHERE role = ?", c.SystemAuthKeyspaceName)
	if err := c.Session.Query(query, roleName).Scan(
		&role.Role,
		&role.CanLogin,
		&role.IsSuperuser,
		&role.MemberOf,
	); err != nil {
		return Role{}, err
	}
	return role, nil
}

func (c *Cluster) CreateRole(role Role) error {
	if err := validateRoleName(role.Role); err != nil {
		return err
	}
	query := fmt.Sprintf(`CREATE ROLE '%s' WITH LOGIN = %v AND SUPERUSER = %v`, role.Role, role.CanLogin, role.IsSuperuser)
	return c.Session.Query(query).Exec()
}

func (c *Cluster) UpdateRole(role Role) error {
	query := fmt.Sprintf(`ALTER ROLE '%s' WITH LOGIN = %v AND SUPERUSER = %v`, role.Role, role.CanLogin, role.IsSuperuser)
	return c.Session.Query(query).Exec()
}

func (c *Cluster) DeleteRole(role Role) error {
	query := fmt.Sprintf(`DROP ROLE '%s'`, role.Role)
	return c.Session.Query(query).Exec()
}

func validateRoleName(name string) error {
	// Only allow alphanumeric and underscore
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return fmt.Errorf("invalid character in role name: %c", r)
		}
	}
	return nil
}
