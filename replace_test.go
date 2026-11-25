// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/highfivelabs/krm-rest-runtime/descriptor"
	"github.com/highfivelabs/krm-rest-runtime/kindtest"
)

// SFREPL (builder, 2026-11-15): only stateless kinds may opt in to
// CREATE OR REPLACE. SFREPL-1 (builder, 2026-11-15, "as recommended") added
// the kinds that hold definitions, not data: View, Pipe, NetworkPolicy,
// PasswordPolicy, Streamlit, Notebook and CodeBundle. TestEveryKindOnFake re-creates each of them in place
// once opted in, and checks that without the opt-in a change only reports
// drift.
var replaceable = []string{
	"Alert", "CodeBundle", "Function", "NetworkPolicy", "NetworkRule", "Notebook",
	"PasswordPolicy", "Pipe", "Procedure", "Stream", "Streamlit", "View",
}

// Each replaceable kind, re-created in place on the fake: a change without
// the opt-in only reports drift; with it, the object is replaced and drift is
// repaired the same way; a new name (or new arguments) is refused. The
// generated samples of some set only fields that name the object, so each
// gets an optional string field of its own CRD to change.
func TestReplaceableKindsOnFake(t *testing.T) {
	extra := map[string]map[string]any{
		"Function":  {"body": "select 1"},
		"Procedure": {"comment": "example-comment"},
		"Stream":    {"comment": "example-comment"},
		// SFREPL-1's kinds whose samples set only naming fields.
		"CodeBundle":     {"comment": "example-comment"},
		"NetworkPolicy":  {"comment": "example-comment"},
		"Notebook":       {"comment": "example-comment"},
		"PasswordPolicy": {"comment": "example-comment"},
		"Streamlit":      {"comment": "example-comment"},
	}
	var doc map[string]any
	if err := yaml.Unmarshal(Types, &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range doc["kinds"].([]any) {
		k := k.(map[string]any)
		for f, v := range extra[k["kind"].(string)] {
			k["sample"].(map[string]any)[f] = v
		}
	}
	types, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	kindtest.Replaces(t, types, Vendor, replaceable...)
}

// Kinds that hold data: never re-created in place.
var neverReplaced = []string{"Table", "DynamicTable", "IcebergTable", "Stage", "ExternalVolume"}

// SFREPL-1 (builder, 2026-11-15): kinds that cannot be updated and stay
// report-only, since replacing them would drop grants, the secret value, the
// sequence position, the integration's trust or external IDs, the images, or
// the index.
var reportOnly = []string{
	"AccountRole", "DatabaseRole", "Secret", "Sequence",
	"CatalogIntegration", "NotificationIntegration", "ImageRepository", "CortexSearchService",
}

func TestOnlyStatelessKindsCanBeReplaced(t *testing.T) {
	f, err := descriptor.Parse(Types)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, k := range f.Kinds {
		if k.Replace != nil {
			got = append(got, k.Kind)
			if k.Replace.Query["createMode"] != "orReplace" {
				t.Errorf("%s: replace query %v, want createMode=orReplace", k.Kind, k.Replace.Query)
			}
		}
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(replaceable, ",") {
		t.Fatalf("kinds with replace: %v, want %v", got, replaceable)
	}
}

// A table (or any kind not allowed) has no spec.replaceOnChange in its CRD,
// so the API server does not accept the field, and the reconciler refuses it
// if it arrives anyway.
func TestDataKindsCannotOptInToReplace(t *testing.T) {
	kindtest.RefusesReplace(t, Types, Vendor, neverReplaced...)
	kindtest.RefusesReplace(t, Types, Vendor, reportOnly...)

	allowed := map[string]bool{}
	for _, k := range replaceable {
		allowed[k] = true
	}
	files, _ := filepath.Glob("config/crds/*.yaml")
	for _, file := range files {
		if strings.HasSuffix(file, "kustomization.yaml") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var crd struct {
			Spec struct {
				Names    struct{ Kind string }
				Versions []struct {
					Schema struct {
						OpenAPIV3Schema struct {
							Properties map[string]struct {
								Properties            map[string]any `json:"properties"`
								PreserveUnknownFields bool           `json:"x-kubernetes-preserve-unknown-fields"`
							} `json:"properties"`
						} `json:"openAPIV3Schema"`
					}
				}
			}
		}
		if err := yaml.Unmarshal(data, &crd); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		kind := crd.Spec.Names.Kind
		for _, v := range crd.Spec.Versions {
			spec, ok := v.Schema.OpenAPIV3Schema.Properties["spec"]
			if !ok {
				continue // the Connection
			}
			_, has := spec.Properties["replaceOnChange"]
			if has != allowed[kind] {
				t.Errorf("%s: spec.replaceOnChange in the CRD = %v, want %v", kind, has, allowed[kind])
			}
			if spec.PreserveUnknownFields {
				t.Errorf("%s: spec keeps unknown fields, so replaceOnChange would not be rejected", kind)
			}
		}
	}
}
