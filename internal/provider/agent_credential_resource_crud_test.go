package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pgbeam "go.pgbeam.com/sdk"
)

// newAgentCredentialResource wires an agentCredentialResource to a client
// pointing at srv.
func newAgentCredentialResource(t *testing.T, srv *httptest.Server) *agentCredentialResource {
	t.Helper()
	r := NewAgentCredentialResource().(*agentCredentialResource)
	resp := &resource.ConfigureResponse{}
	client := pgbeam.NewClient(&pgbeam.ClientOptions{APIKey: "pgb_test", BaseURL: srv.URL})
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}
	if r.client == nil {
		t.Fatal("Configure did not set the client")
	}
	return r
}

// seedAgentCredentialModel builds a state model carrying the given IDs plus the
// one-time secrets, mimicking state written by a prior Create.
func seedAgentCredentialModel(id string) *agentCredentialResourceModel {
	return &agentCredentialResourceModel{
		ID:               types.StringValue(id),
		ProjectID:        types.StringValue("prj_1"),
		ConnectionString: types.StringValue("postgres://agent:one-time@prj-1.proxy.pgbeam.app/db"),
		McpURL:           types.StringValue("https://prj-1.proxy.pgbeam.app/mcp"),
		McpToken:         types.StringValue("mcp_one_time_token"),
	}
}

const agentCredentialJSON = `{
	"id": "agt_1",
	"project_id": "prj_1",
	"policy_profile_id": "pol_1",
	"name": "reporting-agent",
	"pg_username": "agent_reporting",
	"status": "active",
	"principal_type": "agent",
	"auth_method": "scram-sha-256",
	"created_at": "2026-01-01T00:00:00Z",
	"updated_at": "2026-01-02T00:00:00Z"
}`

// TestMapAgentCredentialToState_FullObject covers the SDK-to-state transform
// with every optional field populated.
func TestMapAgentCredentialToState_FullObject(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	lastUsed := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	expires := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	principal := pgbeam.AgentCredentialPrincipalTypeAgent
	authMethod := pgbeam.ScramSha256

	c := &pgbeam.AgentCredential{
		Id:              "agt_1",
		ProjectId:       "prj_1",
		PolicyProfileId: "pol_1",
		Name:            "reporting-agent",
		PgUsername:      "agent_reporting",
		Status:          pgbeam.AgentCredentialStatusActive,
		PrincipalType:   &principal,
		AuthMethod:      &authMethod,
		LastUsedAt:      &lastUsed,
		ExpiresAt:       &expires,
		CreatedAt:       created,
		UpdatedAt:       updated,
	}

	var state agentCredentialResourceModel
	r := &agentCredentialResource{}
	r.mapAgentCredentialToState(&state, c)

	if state.ProjectID.ValueString() != "prj_1" {
		t.Errorf("ProjectID = %q", state.ProjectID.ValueString())
	}
	if state.PolicyProfileID.ValueString() != "pol_1" {
		t.Errorf("PolicyProfileID = %q", state.PolicyProfileID.ValueString())
	}
	if state.Name.ValueString() != "reporting-agent" {
		t.Errorf("Name = %q", state.Name.ValueString())
	}
	if state.PgUsername.ValueString() != "agent_reporting" {
		t.Errorf("PgUsername = %q", state.PgUsername.ValueString())
	}
	if state.Status.ValueString() != "active" {
		t.Errorf("Status = %q", state.Status.ValueString())
	}
	if state.PrincipalType.ValueString() != "agent" {
		t.Errorf("PrincipalType = %q", state.PrincipalType.ValueString())
	}
	if state.AuthMethod.ValueString() != "scram-sha-256" {
		t.Errorf("AuthMethod = %q", state.AuthMethod.ValueString())
	}
	if state.LastUsedAt.ValueString() != lastUsed.Format(time.RFC3339) {
		t.Errorf("LastUsedAt = %q", state.LastUsedAt.ValueString())
	}
	if state.ExpiresAt.ValueString() != expires.Format(time.RFC3339) {
		t.Errorf("ExpiresAt = %q", state.ExpiresAt.ValueString())
	}
	if state.CreatedAt.ValueString() != created.Format(time.RFC3339) {
		t.Errorf("CreatedAt = %q", state.CreatedAt.ValueString())
	}
	if state.UpdatedAt.ValueString() != updated.Format(time.RFC3339) {
		t.Errorf("UpdatedAt = %q", state.UpdatedAt.ValueString())
	}
}

// TestMapAgentCredentialToState_NilOptionals verifies nullable SDK fields map
// to null Terraform values rather than panicking.
func TestMapAgentCredentialToState_NilOptionals(t *testing.T) {
	t.Parallel()

	c := &pgbeam.AgentCredential{
		Id:              "agt_2",
		ProjectId:       "prj_1",
		PolicyProfileId: "pol_1",
		Name:            "minimal",
		PgUsername:      "agent_minimal",
		Status:          pgbeam.AgentCredentialStatusActive,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	var state agentCredentialResourceModel
	r := &agentCredentialResource{}
	r.mapAgentCredentialToState(&state, c)

	if !state.PrincipalType.IsNull() {
		t.Error("PrincipalType should be null")
	}
	if !state.AuthMethod.IsNull() {
		t.Error("AuthMethod should be null")
	}
	if !state.LastUsedAt.IsNull() {
		t.Error("LastUsedAt should be null")
	}
	if !state.ExpiresAt.IsNull() {
		t.Error("ExpiresAt should be null")
	}
	// The mapper never touches the one-time secrets, so they keep their zero
	// (null) value here. Create/Update set them explicitly.
	if !state.ConnectionString.IsNull() {
		t.Error("ConnectionString should be untouched (null)")
	}
	if !state.McpToken.IsNull() {
		t.Error("McpToken should be untouched (null)")
	}
}

// TestAgentCredentialResource_Create_StoresOneTimeSecrets drives Create against
// a mocked API and asserts the one-time secrets from the create response
// (connection string, MCP URL, MCP token) are persisted into state. This is the
// only moment the API ever returns them.
func TestAgentCredentialResource_Create_StoresOneTimeSecrets(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/projects/prj_1/agents" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"credential": ` + agentCredentialJSON + `,
			"connection_string": "postgres://agent:one-time@prj-1.proxy.pgbeam.app/db",
			"mcp_url": "https://prj-1.proxy.pgbeam.app/mcp",
			"mcp_token": "mcp_one_time_token"
		}`))
	}))
	defer srv.Close()

	r := newAgentCredentialResource(t, srv)
	s := resourceSchema(t, NewAgentCredentialResource())

	plan := tfsdk.Plan{Schema: s}
	if d := plan.Set(context.Background(), &agentCredentialResourceModel{
		ProjectID:       types.StringValue("prj_1"),
		PolicyProfileID: types.StringValue("pol_1"),
		Name:            types.StringValue("reporting-agent"),
	}); d.HasError() {
		t.Fatalf("seed plan: %v", d)
	}

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{Plan: plan}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", resp.Diagnostics)
	}

	var out agentCredentialResourceModel
	if d := resp.State.Get(context.Background(), &out); d.HasError() {
		t.Fatalf("create state: %v", d)
	}
	if out.ID.ValueString() != "agt_1" {
		t.Errorf("ID = %q, want agt_1", out.ID.ValueString())
	}
	if out.ConnectionString.ValueString() != "postgres://agent:one-time@prj-1.proxy.pgbeam.app/db" {
		t.Errorf("ConnectionString = %q", out.ConnectionString.ValueString())
	}
	if out.McpURL.ValueString() != "https://prj-1.proxy.pgbeam.app/mcp" {
		t.Errorf("McpURL = %q", out.McpURL.ValueString())
	}
	if out.McpToken.ValueString() != "mcp_one_time_token" {
		t.Errorf("McpToken = %q", out.McpToken.ValueString())
	}
	if out.PgUsername.ValueString() != "agent_reporting" {
		t.Errorf("PgUsername = %q", out.PgUsername.ValueString())
	}
}

// TestAgentCredentialResource_Read_PreservesOneTimeSecrets asserts a refresh
// does not clobber the stored one-time secrets even though the read response
// never contains them.
func TestAgentCredentialResource_Read_PreservesOneTimeSecrets(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/projects/prj_1/agents/agt_1" {
			t.Errorf("path = %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(agentCredentialJSON))
	}))
	defer srv.Close()

	r := newAgentCredentialResource(t, srv)
	s := resourceSchema(t, NewAgentCredentialResource())

	state := tfsdk.State{Schema: s}
	if d := state.Set(context.Background(), seedAgentCredentialModel("agt_1")); d.HasError() {
		t.Fatalf("seed state: %v", d)
	}

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s}}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
	}

	var out agentCredentialResourceModel
	if d := resp.State.Get(context.Background(), &out); d.HasError() {
		t.Fatalf("read state: %v", d)
	}
	if out.Name.ValueString() != "reporting-agent" {
		t.Errorf("Name = %q", out.Name.ValueString())
	}
	if out.ConnectionString.ValueString() != "postgres://agent:one-time@prj-1.proxy.pgbeam.app/db" {
		t.Errorf("ConnectionString lost on refresh: %q", out.ConnectionString.ValueString())
	}
	if out.McpToken.ValueString() != "mcp_one_time_token" {
		t.Errorf("McpToken lost on refresh: %q", out.McpToken.ValueString())
	}
}

// TestAgentCredentialResource_Read_NotFound asserts a 404 removes the resource
// from state (drift handling) rather than erroring.
func TestAgentCredentialResource_Read_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"gone"}}`))
	}))
	defer srv.Close()

	r := newAgentCredentialResource(t, srv)
	s := resourceSchema(t, NewAgentCredentialResource())

	state := tfsdk.State{Schema: s}
	if d := state.Set(context.Background(), seedAgentCredentialModel("agt_gone")); d.HasError() {
		t.Fatalf("seed state: %v", d)
	}

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s}}
	if d := resp.State.Set(context.Background(), seedAgentCredentialModel("agt_gone")); d.HasError() {
		t.Fatalf("seed resp state: %v", d)
	}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) on 404")
	}
}

// TestAgentCredentialResource_Delete_NotFound asserts revoking an already-gone
// credential succeeds (idempotent delete).
func TestAgentCredentialResource_Delete_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", req.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	r := newAgentCredentialResource(t, srv)
	s := resourceSchema(t, NewAgentCredentialResource())

	state := tfsdk.State{Schema: s}
	if d := state.Set(context.Background(), seedAgentCredentialModel("agt_gone")); d.HasError() {
		t.Fatalf("seed state: %v", d)
	}

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s}}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete should ignore 404, got: %v", resp.Diagnostics)
	}
}

// TestAgentCredentialResource_ImportState drives import by
// project_id/agent_id. Imported credentials have no one-time secrets: the API
// only returns them at create/rotate time, so they stay null.
func TestAgentCredentialResource_ImportState(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/projects/prj_1/agents/agt_1" {
			t.Errorf("path = %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(agentCredentialJSON))
	}))
	defer srv.Close()

	r := newAgentCredentialResource(t, srv)
	s := resourceSchema(t, NewAgentCredentialResource())

	resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: s}}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "prj_1/agt_1"}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState diagnostics: %v", resp.Diagnostics)
	}

	var out agentCredentialResourceModel
	if d := resp.State.Get(context.Background(), &out); d.HasError() {
		t.Fatalf("import state: %v", d)
	}
	if out.ID.ValueString() != "agt_1" {
		t.Errorf("ID = %q, want agt_1", out.ID.ValueString())
	}
	if out.ProjectID.ValueString() != "prj_1" {
		t.Errorf("ProjectID = %q, want prj_1", out.ProjectID.ValueString())
	}
	if !out.ConnectionString.IsNull() {
		t.Error("imported credential must not carry a connection string")
	}
	if !out.McpToken.IsNull() {
		t.Error("imported credential must not carry an MCP token")
	}
}

// TestAgentCredentialResource_ImportState_BadID asserts a malformed import ID
// yields a clear error.
func TestAgentCredentialResource_ImportState_BadID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newAgentCredentialResource(t, srv)
	s := resourceSchema(t, NewAgentCredentialResource())

	resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: s}}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "missing-separator"}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for malformed import ID")
	}
}
