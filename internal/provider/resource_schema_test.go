package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// resourceSchema returns the schema produced by a resource factory.
func resourceSchema(t *testing.T, r resource.Resource) resschema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// TestAllResourceSchemas_ValidImplementation asserts every registered resource
// produces a schema that the plugin framework considers well-formed. This is
// the guard that catches an OpenAPI/SDK change breaking codegen: a malformed
// attribute (e.g. required+computed, bad nesting) fails here.
func TestAllResourceSchemas_ValidImplementation(t *testing.T) {
	t.Parallel()

	for i, factory := range pgbeamResources() {
		r := factory()
		s := resourceSchema(t, r)
		if diags := s.ValidateImplementation(context.Background()); diags.HasError() {
			t.Errorf("resource %d (%T) schema failed validation: %v", i, r, diags)
		}
	}
}

// TestAllResourceSchemas_HaveID asserts every resource exposes a computed "id"
// attribute, which the framework and import flow rely on.
func TestAllResourceSchemas_HaveID(t *testing.T) {
	t.Parallel()

	for _, factory := range pgbeamResources() {
		r := factory()
		s := resourceSchema(t, r)
		idAttr, ok := s.Attributes["id"]
		if !ok {
			t.Errorf("resource %T missing 'id' attribute", r)
			continue
		}
		if !idAttr.IsComputed() {
			t.Errorf("resource %T 'id' attribute must be computed", r)
		}
	}
}

// TestAllResourceMetadata asserts each resource derives its type name from the
// provider type name prefix, e.g. "pgbeam_project".
func TestAllResourceMetadata(t *testing.T) {
	t.Parallel()

	for _, factory := range pgbeamResources() {
		r := factory()
		resp := &resource.MetadataResponse{}
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "pgbeam"}, resp)
		if resp.TypeName == "" {
			t.Errorf("resource %T produced empty TypeName", r)
		}
		if len(resp.TypeName) < len("pgbeam_") || resp.TypeName[:len("pgbeam_")] != "pgbeam_" {
			t.Errorf("resource %T TypeName = %q, want prefix pgbeam_", r, resp.TypeName)
		}
	}
}

// TestSecretAttributes_Sensitive pins the sensitive marking on every attribute
// that carries a secret: the agent credential's one-time connection string and
// MCP token, the webhook signing secret, and the self-host enrollment token.
// Losing the marking would print secrets in plan output and CI logs.
func TestSecretAttributes_Sensitive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		factory func() resource.Resource
		attrs   []string
	}{
		{NewAgentCredentialResource, []string{"connection_string", "mcp_token"}},
		{NewWebhookEndpointResource, []string{"secret"}},
		{NewSelfHostEnrollmentResource, []string{"token"}},
	}

	for _, tc := range cases {
		r := tc.factory()
		s := resourceSchema(t, r)
		for _, name := range tc.attrs {
			attr, ok := s.Attributes[name]
			if !ok {
				t.Errorf("resource %T missing attribute %q", r, name)
				continue
			}
			if !attr.IsSensitive() {
				t.Errorf("resource %T attribute %q must be sensitive", r, name)
			}
		}
	}
}

// TestProjectResourceSchema_KeyAttributes checks the attribute contract most
// likely to drift on an SDK/OpenAPI change: required inputs, computed outputs,
// and the nested allowed_cidrs block.
func TestProjectResourceSchema_KeyAttributes(t *testing.T) {
	t.Parallel()

	s := resourceSchema(t, NewProjectResource())

	required := []string{"org_id", "name"}
	for _, name := range required {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Errorf("missing attribute %q", name)
			continue
		}
		if !attr.IsRequired() {
			t.Errorf("attribute %q must be required", name)
		}
	}

	computed := []string{"proxy_host", "created_at", "updated_at", "primary_database_id", "database_count"}
	for _, name := range computed {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Errorf("missing attribute %q", name)
			continue
		}
		if !attr.IsComputed() {
			t.Errorf("attribute %q must be computed", name)
		}
	}

	if _, ok := s.Attributes["allowed_cidrs"]; !ok {
		t.Error("missing allowed_cidrs nested attribute")
	}
}
