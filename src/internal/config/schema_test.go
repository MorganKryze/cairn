package config

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The editor schemas in schema/ must describe exactly the keys the structs
// decode: a key added in Go without a schema update fails here.

func yamlTags(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		tag := strings.Split(typ.Field(i).Tag.Get("yaml"), ",")[0]
		if tag != "" && tag != "-" {
			out[tag] = true
		}
	}
	return out
}

func schemaProps(t *testing.T, file string, path ...string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	node := doc
	for _, p := range path {
		next, ok := node[p].(map[string]any)
		if !ok {
			t.Fatalf("%s: no object at %v", file, path)
		}
		node = next
	}
	out := map[string]bool{}
	for k := range node {
		out[k] = true
	}
	return out
}

func TestSchemasMatchStructs(t *testing.T) {
	for _, tc := range []struct {
		typ  reflect.Type
		file string
		path []string
	}{
		{reflect.TypeOf(Site{}), "../../../schema/site.json", []string{"properties"}},
		{reflect.TypeOf(Service{}), "../../../schema/services.json", []string{"items", "properties"}},
		{reflect.TypeOf(CategoryMeta{}), "../../../schema/categories.json", []string{"items", "properties"}},
		{reflect.TypeOf(SitePage{}), "../../../schema/site.json", []string{"properties", "pages", "items", "properties"}},
		{reflect.TypeOf(FooterLink{}), "../../../schema/site.json", []string{"properties", "links", "items", "properties"}},
	} {
		want, got := yamlTags(t, tc.typ), schemaProps(t, tc.file, tc.path...)
		for k := range want {
			if !got[k] {
				t.Errorf("%s: schema is missing key %q of %s", tc.file, k, tc.typ.Name())
			}
		}
		for k := range got {
			if !want[k] {
				t.Errorf("%s: schema declares %q, unknown to %s", tc.file, k, tc.typ.Name())
			}
		}
	}
}
