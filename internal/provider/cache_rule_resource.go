package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pgbeam "github.com/pgbeam/pgbeam-go"
)

var (
	_ resource.Resource                = (*cacheRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*cacheRuleResource)(nil)
	_ resource.ResourceWithImportState = (*cacheRuleResource)(nil)
)

type cacheRuleResource struct {
	client *pgbeam.Client
}

type cacheRuleResourceModel struct {
	ID               types.String  `tfsdk:"id"`
	ProjectID        types.String  `tfsdk:"project_id"`
	DatabaseID       types.String  `tfsdk:"database_id"`
	QueryHash        types.String  `tfsdk:"query_hash"`
	CacheEnabled     types.Bool    `tfsdk:"cache_enabled"`
	CacheTTLSeconds  types.Int64   `tfsdk:"cache_ttl_seconds"`
	CacheSWRSeconds  types.Int64   `tfsdk:"cache_swr_seconds"`
	NormalizedSQL    types.String  `tfsdk:"normalized_sql"`
	QueryType        types.String  `tfsdk:"query_type"`
	CallCount        types.Int64   `tfsdk:"call_count"`
	AvgLatencyMs     types.Float64 `tfsdk:"avg_latency_ms"`
	P95LatencyMs     types.Float64 `tfsdk:"p95_latency_ms"`
	AvgResponseBytes types.Int64   `tfsdk:"avg_response_bytes"`
	StabilityRate    types.Float64 `tfsdk:"stability_rate"`
	Recommendation   types.String  `tfsdk:"recommendation"`
	FirstSeenAt      types.String  `tfsdk:"first_seen_at"`
	LastSeenAt       types.String  `tfsdk:"last_seen_at"`
}

func NewCacheRuleResource() resource.Resource {
	return &cacheRuleResource{}
}

func (r *cacheRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cache_rule"
}

func (r *cacheRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a per-query cache rule for a PgBeam database. Uses PUT to create and update. Deletion disables caching (soft delete).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite ID in the format project_id/database_id/query_hash.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database_id": schema.StringAttribute{
				Description: "Database ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"query_hash": schema.StringAttribute{
				Description: "Hash identifying the normalized SQL query.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cache_enabled": schema.BoolAttribute{
				Description: "Whether caching is enabled for this query.",
				Required:    true,
			},
			"cache_ttl_seconds": schema.Int64Attribute{
				Description: "Cache TTL override in seconds. Null uses database default.",
				Optional:    true,
			},
			"cache_swr_seconds": schema.Int64Attribute{
				Description: "Stale-while-revalidate override in seconds. Null uses database default.",
				Optional:    true,
			},
			"normalized_sql": schema.StringAttribute{
				Description: "The normalized SQL query text.",
				Computed:    true,
			},
			"query_type": schema.StringAttribute{
				Description: "Query type (e.g. SELECT).",
				Computed:    true,
			},
			"call_count": schema.Int64Attribute{
				Description: "Total number of times this query has been executed.",
				Computed:    true,
			},
			"avg_latency_ms": schema.Float64Attribute{
				Description: "Average query latency in milliseconds.",
				Computed:    true,
			},
			"p95_latency_ms": schema.Float64Attribute{
				Description: "95th percentile query latency in milliseconds.",
				Computed:    true,
			},
			"avg_response_bytes": schema.Int64Attribute{
				Description: "Average response size in bytes.",
				Computed:    true,
			},
			"stability_rate": schema.Float64Attribute{
				Description: "Query result stability rate (0.0 to 1.0).",
				Computed:    true,
			},
			"recommendation": schema.StringAttribute{
				Description: "PgBeam's caching recommendation for this query.",
				Computed:    true,
			},
			"first_seen_at": schema.StringAttribute{
				Description: "ISO 8601 timestamp of when this query was first observed.",
				Computed:    true,
			},
			"last_seen_at": schema.StringAttribute{
				Description: "ISO 8601 timestamp of when this query was last observed.",
				Computed:    true,
			},
		},
	}
}

func (r *cacheRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*pgbeam.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *pgbeam.Client, got: %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *cacheRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cacheRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := pgbeam.UpdateCacheRuleRequest{
		CacheEnabled: plan.CacheEnabled.ValueBool(),
	}

	if !plan.CacheTTLSeconds.IsNull() && !plan.CacheTTLSeconds.IsUnknown() {
		ttl := int(plan.CacheTTLSeconds.ValueInt64())
		updateReq.CacheTTLSeconds = &ttl
	}
	if !plan.CacheSWRSeconds.IsNull() && !plan.CacheSWRSeconds.IsUnknown() {
		swr := int(plan.CacheSWRSeconds.ValueInt64())
		updateReq.CacheSWRSeconds = &swr
	}

	projectID := plan.ProjectID.ValueString()
	databaseID := plan.DatabaseID.ValueString()
	queryHash := plan.QueryHash.ValueString()

	rule, err := r.client.CacheRules.Update(ctx, projectID, databaseID, queryHash, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating cache rule", err.Error())
		return
	}

	plan.ID = types.StringValue(projectID + "/" + databaseID + "/" + queryHash)
	r.mapCacheRuleToState(&plan, rule)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cacheRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cacheRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.CacheRules.Get(ctx, state.ProjectID.ValueString(), state.DatabaseID.ValueString(), state.QueryHash.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading cache rule", err.Error())
		return
	}

	r.mapCacheRuleToState(&state, rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cacheRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cacheRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := pgbeam.UpdateCacheRuleRequest{
		CacheEnabled: plan.CacheEnabled.ValueBool(),
	}

	if !plan.CacheTTLSeconds.IsNull() && !plan.CacheTTLSeconds.IsUnknown() {
		ttl := int(plan.CacheTTLSeconds.ValueInt64())
		updateReq.CacheTTLSeconds = &ttl
	}
	if !plan.CacheSWRSeconds.IsNull() && !plan.CacheSWRSeconds.IsUnknown() {
		swr := int(plan.CacheSWRSeconds.ValueInt64())
		updateReq.CacheSWRSeconds = &swr
	}

	rule, err := r.client.CacheRules.Update(ctx, plan.ProjectID.ValueString(), plan.DatabaseID.ValueString(), plan.QueryHash.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating cache rule", err.Error())
		return
	}

	r.mapCacheRuleToState(&plan, rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cacheRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cacheRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.CacheRules.Disable(ctx, state.ProjectID.ValueString(), state.DatabaseID.ValueString(), state.QueryHash.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting cache rule", err.Error())
	}
}

func (r *cacheRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid Import ID", "Expected format: project_id/database_id/query_hash")
		return
	}

	projectID := parts[0]
	databaseID := parts[1]
	queryHash := parts[2]

	rule, err := r.client.CacheRules.Get(ctx, projectID, databaseID, queryHash)
	if err != nil {
		resp.Diagnostics.AddError("Error importing cache rule", err.Error())
		return
	}

	var state cacheRuleResourceModel
	state.ID = types.StringValue(req.ID)
	state.ProjectID = types.StringValue(projectID)
	state.DatabaseID = types.StringValue(databaseID)
	state.QueryHash = types.StringValue(queryHash)
	r.mapCacheRuleToState(&state, rule)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cacheRuleResource) mapCacheRuleToState(state *cacheRuleResourceModel, rule *pgbeam.CacheRule) {
	state.CacheEnabled = types.BoolValue(rule.CacheEnabled)

	if rule.CacheTTLSeconds != nil {
		state.CacheTTLSeconds = types.Int64Value(int64(*rule.CacheTTLSeconds))
	} else {
		state.CacheTTLSeconds = types.Int64Null()
	}

	if rule.CacheSWRSeconds != nil {
		state.CacheSWRSeconds = types.Int64Value(int64(*rule.CacheSWRSeconds))
	} else {
		state.CacheSWRSeconds = types.Int64Null()
	}

	state.NormalizedSQL = types.StringValue(rule.NormalizedSQL)
	state.QueryType = types.StringValue(rule.QueryType)
	state.CallCount = types.Int64Value(rule.CallCount)
	state.AvgLatencyMs = types.Float64Value(rule.AvgLatencyMs)
	state.P95LatencyMs = types.Float64Value(rule.P95LatencyMs)
	state.AvgResponseBytes = types.Int64Value(rule.AvgResponseBytes)
	state.StabilityRate = types.Float64Value(rule.StabilityRate)
	state.Recommendation = types.StringValue(rule.Recommendation)
	state.FirstSeenAt = types.StringValue(rule.FirstSeenAt)
	state.LastSeenAt = types.StringValue(rule.LastSeenAt)
}
