// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
)

func AuthLoginUserpassSchema() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		Description: "Login to ScyllaDB using the userpass method",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				Description: "Login with username",
				Required:    true,
			},
			"password": schema.StringAttribute{
				Description: "Login with password",
				Required:    true,
				Sensitive:   true,
			},
		},
	}
}
