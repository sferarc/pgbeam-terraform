package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pgbeam "go.pgbeam.com/sdk"
)

// newProjectResource wires a projectResource to a client pointing at srv.
func newProjectResource(t *testing.T, srv *httptest.Server) *projectResource {
	t.Helper()
	r := NewProjectResource().(*projectResource)
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

// seedModel builds a projectResourceModel carrying the given ID with the
// nullable List attributes typed correctly, so tfsdk.State.Set accepts it.
func seedModel(id string) *projectResourceModel {
	return &projectResourceModel{
		ID:           types.StringValue(id),
		Tags:         types.ListNull(types.StringType),
		AllowedCidrs: types.ListNull(types.ObjectType{AttrTypes: allowedCidrsElemAttrTypes()}),
	}
}

// TestProjectResource_Configure_WrongType asserts a clear error when the
// framework passes an unexpected ProviderData type.
func TestProjectResource_Configure_WrongType(t *testing.T) {
	t.Parallel()

	r := NewProjectResource().(*projectResource)
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a client"}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for wrong ProviderData type")
	}
}

// TestProjectResource_Configure_NilData is a no-op (framework calls Configure
// with nil during early phases) and must not error or set a client.
func TestProjectResource_Configure_NilData(t *testing.T) {
	t.Parallel()

	r := NewProjectResource().(*projectResource)
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if r.client != nil {
		t.Error("client should remain nil when ProviderData is nil")
	}
}

// TestMapProjectToState covers the transform that turns an SDK Project into
// Terraform state. This is the code most exposed to SDK field drift.
func TestMapProjectToState_FullObject(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	desc := "a project"
	proxyHost := "myproj.proxy.pgbeam.app"
	var qps int32 = 100
	var burst int32 = 200
	var maxConn int32 = 50
	dbCount := 3
	activeConn := 7
	profileID := "pol_1"
	agentsDisabled := true
	cloud := pgbeam.ProjectCloudAws
	label := "office"

	p := &pgbeam.Project{
		Id:                     "prj_1",
		OrgId:                  "org_1",
		Name:                   "My Project",
		Description:            &desc,
		Tags:                   &[]string{"a", "b"},
		Cloud:                  &cloud,
		ProxyHost:              &proxyHost,
		QueriesPerSecond:       &qps,
		BurstSize:              &burst,
		MaxConnections:         &maxConn,
		AllowedCidrs:           &[]pgbeam.CidrEntry{{Cidr: "10.0.0.0/8", Label: &label}, {Cidr: "192.168.0.0/16"}},
		DefaultPolicyProfileId: &profileID,
		AgentsDisabled:         &agentsDisabled,
		DatabaseCount:          &dbCount,
		ActiveConnections:      &activeConn,
		Status:                 pgbeam.ProjectStatusActive,
		CreatedAt:              created,
		UpdatedAt:              updated,
	}

	var state projectResourceModel
	var diags diag.Diagnostics
	r := &projectResource{}
	r.mapProjectToState(context.Background(), &state, p, &diags)
	if diags.HasError() {
		t.Fatalf("mapProjectToState diagnostics: %v", diags)
	}

	if state.OrgID.ValueString() != "org_1" {
		t.Errorf("OrgID = %q, want org_1", state.OrgID.ValueString())
	}
	if state.Name.ValueString() != "My Project" {
		t.Errorf("Name = %q", state.Name.ValueString())
	}
	if state.Description.ValueString() != "a project" {
		t.Errorf("Description = %q", state.Description.ValueString())
	}
	if state.Cloud.ValueString() != "aws" {
		t.Errorf("Cloud = %q, want aws", state.Cloud.ValueString())
	}
	if state.ProxyHost.ValueString() != proxyHost {
		t.Errorf("ProxyHost = %q", state.ProxyHost.ValueString())
	}
	if state.QueriesPerSecond.ValueInt64() != 100 {
		t.Errorf("QueriesPerSecond = %d, want 100", state.QueriesPerSecond.ValueInt64())
	}
	if state.BurstSize.ValueInt64() != 200 {
		t.Errorf("BurstSize = %d, want 200", state.BurstSize.ValueInt64())
	}
	if state.MaxConnections.ValueInt64() != 50 {
		t.Errorf("MaxConnections = %d, want 50", state.MaxConnections.ValueInt64())
	}
	if state.DatabaseCount.ValueInt64() != 3 {
		t.Errorf("DatabaseCount = %d, want 3", state.DatabaseCount.ValueInt64())
	}
	if state.ActiveConnections.ValueInt64() != 7 {
		t.Errorf("ActiveConnections = %d, want 7", state.ActiveConnections.ValueInt64())
	}
	if state.DefaultPolicyProfileID.ValueString() != "pol_1" {
		t.Errorf("DefaultPolicyProfileID = %q", state.DefaultPolicyProfileID.ValueString())
	}
	if !state.AgentsDisabled.ValueBool() {
		t.Error("AgentsDisabled should be true")
	}
	if state.Status.ValueString() != "active" {
		t.Errorf("Status = %q, want active", state.Status.ValueString())
	}
	if state.CreatedAt.ValueString() != created.Format(time.RFC3339) {
		t.Errorf("CreatedAt = %q", state.CreatedAt.ValueString())
	}
	if state.UpdatedAt.ValueString() != updated.Format(time.RFC3339) {
		t.Errorf("UpdatedAt = %q", state.UpdatedAt.ValueString())
	}

	// Tags list round-trips.
	var tags []string
	if d := state.Tags.ElementsAs(context.Background(), &tags, false); d.HasError() {
		t.Fatalf("tags ElementsAs: %v", d)
	}
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("Tags = %v, want [a b]", tags)
	}

	// allowed_cidrs nested list round-trips, including the nil label case.
	var cidrs []allowedCidrsElemModel
	if d := state.AllowedCidrs.ElementsAs(context.Background(), &cidrs, false); d.HasError() {
		t.Fatalf("allowed_cidrs ElementsAs: %v", d)
	}
	if len(cidrs) != 2 {
		t.Fatalf("allowed_cidrs len = %d, want 2", len(cidrs))
	}
	if cidrs[0].Cidr.ValueString() != "10.0.0.0/8" || cidrs[0].Label.ValueString() != "office" {
		t.Errorf("cidr[0] = %+v", cidrs[0])
	}
	if cidrs[1].Cidr.ValueString() != "192.168.0.0/16" || !cidrs[1].Label.IsNull() {
		t.Errorf("cidr[1] label should be null: %+v", cidrs[1])
	}
}

// TestMapProjectToState_NilOptionals verifies nullable SDK fields map to null
// (or documented zero) Terraform values rather than panicking.
func TestMapProjectToState_NilOptionals(t *testing.T) {
	t.Parallel()

	p := &pgbeam.Project{
		Id:        "prj_2",
		OrgId:     "org_2",
		Name:      "Minimal",
		Status:    pgbeam.ProjectStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	var state projectResourceModel
	var diags diag.Diagnostics
	r := &projectResource{}
	r.mapProjectToState(context.Background(), &state, p, &diags)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if !state.Description.IsNull() {
		t.Error("Description should be null")
	}
	if !state.Cloud.IsNull() {
		t.Error("Cloud should be null")
	}
	if !state.ProxyHost.IsNull() {
		t.Error("ProxyHost should be null")
	}
	if !state.DefaultPolicyProfileID.IsNull() {
		t.Error("DefaultPolicyProfileID should be null")
	}
	if !state.AgentsDisabled.IsNull() {
		t.Error("AgentsDisabled should be null")
	}
	// Numeric outputs default to 0 (not null) per the generated mapper.
	if state.QueriesPerSecond.ValueInt64() != 0 {
		t.Errorf("QueriesPerSecond = %d, want 0", state.QueriesPerSecond.ValueInt64())
	}
	if state.DatabaseCount.ValueInt64() != 0 {
		t.Errorf("DatabaseCount = %d, want 0", state.DatabaseCount.ValueInt64())
	}
}

// TestProjectResource_Read drives the Read path against a mocked API and
// asserts the API response is mapped into fresh state.
func TestProjectResource_Read(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/projects/prj_read" {
			t.Errorf("path = %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"prj_read","org_id":"org_1","name":"Read Me","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	r := newProjectResource(t, srv)
	s := resourceSchema(t, NewProjectResource())

	state := tfsdk.State{Schema: s}
	if d := state.Set(context.Background(), seedModel("prj_read")); d.HasError() {
		t.Fatalf("seed state: %v", d)
	}

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s}}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
	}

	var out projectResourceModel
	if d := resp.State.Get(context.Background(), &out); d.HasError() {
		t.Fatalf("read state: %v", d)
	}
	if out.Name.ValueString() != "Read Me" {
		t.Errorf("Name = %q, want Read Me", out.Name.ValueString())
	}
}

// TestProjectResource_Read_NotFound asserts a 404 removes the resource from
// state (drift handling), rather than erroring.
func TestProjectResource_Read_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"gone"}}`))
	}))
	defer srv.Close()

	r := newProjectResource(t, srv)
	s := resourceSchema(t, NewProjectResource())

	state := tfsdk.State{Schema: s}
	if d := state.Set(context.Background(), seedModel("prj_gone")); d.HasError() {
		t.Fatalf("seed state: %v", d)
	}

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s}}
	// Seed the response state so RemoveResource has something to clear.
	if d := resp.State.Set(context.Background(), seedModel("prj_gone")); d.HasError() {
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

// TestProjectResource_Delete_NotFound asserts deleting an already-gone project
// succeeds (idempotent delete).
func TestProjectResource_Delete_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", req.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	r := newProjectResource(t, srv)
	s := resourceSchema(t, NewProjectResource())

	state := tfsdk.State{Schema: s}
	if d := state.Set(context.Background(), seedModel("prj_gone")); d.HasError() {
		t.Fatalf("seed state: %v", d)
	}

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s}}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete should ignore 404, got: %v", resp.Diagnostics)
	}
}

// TestProjectResource_ImportState drives import by ID, which fetches the
// project and populates state.
func TestProjectResource_ImportState(t *testing.T) {
	t.Parallel()

	// The imported project carries tags and allowed_cidrs so the state mapper
	// sets those List attributes with concrete element types. See
	// TestProjectResource_ImportState_NullListsGap for the null-list case.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"prj_imp","org_id":"org_1","name":"Imported","status":"active","tags":["x"],"allowed_cidrs":[{"cidr":"10.0.0.0/8"}],"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	r := newProjectResource(t, srv)
	s := resourceSchema(t, NewProjectResource())

	resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: s}}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "prj_imp"}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState diagnostics: %v", resp.Diagnostics)
	}

	var out projectResourceModel
	if d := resp.State.Get(context.Background(), &out); d.HasError() {
		t.Fatalf("import state: %v", d)
	}
	if out.ID.ValueString() != "prj_imp" {
		t.Errorf("ID = %q, want prj_imp", out.ID.ValueString())
	}
	if out.Name.ValueString() != "Imported" {
		t.Errorf("Name = %q, want Imported", out.Name.ValueString())
	}
}

// TestProjectResource_ImportState_NullLists verifies that importing a project
// whose tags/allowed_cidrs are null succeeds. Previously mapProjectToState left
// those List attributes as untyped zero values when both the incoming (empty)
// state and the API response were null, producing a "MISSING TYPE" state
// conversion error. The generator now always sets typed null lists.
func TestProjectResource_ImportState_NullLists(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"prj_imp","org_id":"org_1","name":"Imported","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	r := newProjectResource(t, srv)
	s := resourceSchema(t, NewProjectResource())

	resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: s}}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "prj_imp"}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState diagnostics: %v", resp.Diagnostics)
	}

	var out projectResourceModel
	if d := resp.State.Get(context.Background(), &out); d.HasError() {
		t.Fatalf("import state: %v", d)
	}
	if !out.Tags.IsNull() {
		t.Errorf("Tags = %v, want null", out.Tags)
	}
	if out.Tags.ElementType(context.Background()) == nil {
		t.Error("Tags null list has no element type (MISSING TYPE)")
	}
	if !out.AllowedCidrs.IsNull() {
		t.Errorf("AllowedCidrs = %v, want null", out.AllowedCidrs)
	}
	if out.AllowedCidrs.ElementType(context.Background()) == nil {
		t.Error("AllowedCidrs null list has no element type (MISSING TYPE)")
	}
}
