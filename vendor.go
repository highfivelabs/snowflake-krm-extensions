// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"net/http"

	"github.com/highfivelabs/krm-rest-runtime/rest"
)

// Vendor is how the manager reaches Snowflake.
var Vendor = &rest.Vendor{
	Name:           "snowflake",
	DefaultBaseURL: "", // every Connection names its Snowflake
	CredentialKeys: []string{"token"},
	Authorize: func(r *http.Request, c map[string]string) {
		r.Header.Set("Authorization", "Bearer "+c["token"])
		t := c["tokenType"]
		if t == "" {
			t = "PROGRAMMATIC_ACCESS_TOKEN"
		}
		r.Header.Set("X-Snowflake-Authorization-Token-Type", t)
	},
}
