package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pgbeam "github.com/pgbeam/pgbeam-go"
)

var (
	_ resource.Resource                = (*projectResource)(nil)
	_ resource.ResourceWithConfigure   = (*projectResource)(nil)
	_ resource.ResourceWithImportState = (*projectResource)(nil)
)

type projectResource struct {
	client *pgbeam.Client
}

type projectResourceModel struct {
	ID                types.String `tfsdk:"id"`
	OrgID             types.String `tfsdk:"org_id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	Tags              types.List   `tfsdk:"tags"`
	Cloud             types.String `tfsdk:"cloud"`
	QueriesPerSecond  types.Int64  `tfsdk:"queries_per_second"`
	BurstSize         types.Int64  `tfsdk:"burst_size"`
	MaxConnections    types.Int64  `tfsdk:"max_connections"`
	Database          types.Object `tfsdk:"database"`
	ProxyHost         types.String `tfsdk:"proxy_host"`
	Status            types.String `tfsdk:"status"`
	DatabaseCount     types.Int64  `tfsdk:"database_count"`
	ActiveConnections types.Int64  `tfsdk:"active_connections"`
	PrimaryDatabaseID types.String `tfsdk:"primary_database_id"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

type projectDatabaseModel struct {
	Host        types.String `tfsdk:"host"`
	Port        types.Int64  `tfsdk:"port"`
	Name        types.String `tfsdk:"name"`
	Username    types.String `tfsdk:"username"`
	Password    types.String `tfsdk:"password"`
	SSLMode     types.String `tfsdk:"ssl_mode"`
	Role        types.String `tfsdk:"role"`
	PoolRegion  types.String `tfsdk:"pool_region"`
	CacheConfig types.Object `tfsdk:"cache_config"`
	PoolConfig  types.Object `tfsdk:"pool_config"`
}

type cacheConfigModel struct {
	Enabled    types.Bool  `tfsdk:"enabled"`
	TTLSeconds types.Int64 `tfsdk:"ttl_seconds"`
	MaxEntries types.Int64 `tfsdk:"max_entries"`
	SWRSeconds types.Int64 `tfsdk:"swr_seconds"`
}

type poolConfigModel struct {
	PoolSize    types.Int64  `tfsdk:"pool_size"`
	MinPoolSize types.Int64  `tfsdk:"min_pool_size"`
	PoolMode    types.String `tfsdk:"pool_mode"`
}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func cacheConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":     types.BoolType,
		"ttl_seconds": types.Int64Type,
		"max_entries": types.Int64Type,
		"swr_seconds": types.Int64Type,
	}
}

func poolConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"pool_size":     types.Int64Type,
		"min_pool_size": types.Int64Type,
		"pool_mode":     types.StringType,
	}
}

func projectDatabaseAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"host":         types.StringType,
		"port":         types.Int64Type,
		"name":         types.StringType,
		"username":     types.StringType,
		"password":     types.StringType,
		"ssl_mode":     types.StringType,
		"role":         types.StringType,
		"pool_region":  types.StringType,
		"cache_config": types.ObjectType{AttrTypes: cacheConfigAttrTypes()},
		"pool_config":  types.ObjectType{AttrTypes: poolConfigAttrTypes()},
	}
}

func cacheConfigSchemaBlock() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: "Query cache configuration.",
		Optional:    true,
		Computed:    true,
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				Description: "Whether query caching is enabled.",
				Optional:    true,
				Computed:    true,
			},
			"ttl_seconds": schema.Int64Attribute{
				Description: "Cache time-to-live in seconds.",
				Optional:    true,
				Computed:    true,
			},
			"max_entries": schema.Int64Attribute{
				Description: "Maximum number of cache entries.",
				Optional:    true,
				Computed:    true,
			},
			"swr_seconds": schema.Int64Attribute{
				Description: "Stale-while-revalidate window in seconds.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func poolConfigSchemaBlock() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: "Connection pool configuration.",
		Optional:    true,
		Computed:    true,
		Attributes: map[string]schema.Attribute{
			"pool_size": schema.Int64Attribute{
				Description: "Maximum pool size.",
				Optional:    true,
				Computed:    true,
			},
			"min_pool_size": schema.Int64Attribute{
				Description: "Minimum pool size.",
				Optional:    true,
				Computed:    true,
			},
			"pool_mode": schema.StringAttribute{
				Description: "Pool mode: transaction or session.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a PgBeam project. Projects are the top-level organizational unit, each with a unique proxy hostname. Created atomically with a primary upstream database.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Project ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org_id": schema.StringAttribute{
				Description: "Organization ID that owns this project.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-readable project name (1-100 characters).",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Optional project description (up to 500 characters).",
				Optional:    true,
			},
			"tags": schema.ListAttribute{
				Description: "Optional user-defined labels (max 10, each up to 50 characters).",
				Optional:    true,
				ElementType: types.StringType,
			},
			"cloud": schema.StringAttribute{
				Description: "Cloud provider: aws, azure, or gcp. Defaults to aws.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"queries_per_second": schema.Int64Attribute{
				Description: "Rate limit: max sustained queries per second (0 = unlimited).",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"burst_size": schema.Int64Attribute{
				Description: "Rate limit: burst allowance above sustained QPS.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_connections": schema.Int64Attribute{
				Description: "Max concurrent connections (0 = unlimited).",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"database": schema.SingleNestedAttribute{
				Description: "Primary database configuration, created atomically with the project.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{
						Description: "Upstream PostgreSQL host.",
						Required:    true,
					},
					"port": schema.Int64Attribute{
						Description: "Upstream PostgreSQL port (1-65535).",
						Required:    true,
					},
					"name": schema.StringAttribute{
						Description: "PostgreSQL database name.",
						Required:    true,
					},
					"username": schema.StringAttribute{
						Description: "PostgreSQL username.",
						Required:    true,
					},
					"password": schema.StringAttribute{
						Description: "PostgreSQL password (encrypted at rest).",
						Required:    true,
						Sensitive:   true,
					},
					"ssl_mode": schema.StringAttribute{
						Description: "SSL connection mode: disable, allow, prefer, require, verify-ca, or verify-full.",
						Optional:    true,
						Computed:    true,
					},
					"role": schema.StringAttribute{
						Description: "Database role: primary or replica.",
						Optional:    true,
						Computed:    true,
					},
					"pool_region": schema.StringAttribute{
						Description: "Region for the connection pool (e.g. us-east-1).",
						Optional:    true,
					},
					"cache_config": cacheConfigSchemaBlock(),
					"pool_config":  poolConfigSchemaBlock(),
				},
			},
			"proxy_host": schema.StringAttribute{
				Description: "PgBeam proxy hostname for this project.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Project status: active, suspended, or deleted.",
				Computed:    true,
			},
			"database_count": schema.Int64Attribute{
				Description: "Number of databases attached to this project.",
				Computed:    true,
			},
			"active_connections": schema.Int64Attribute{
				Description: "Current active connections from latest metrics.",
				Computed:    true,
			},
			"primary_database_id": schema.StringAttribute{
				Description: "ID of the primary database created with this project.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "ISO 8601 creation timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "ISO 8601 last-update timestamp.",
				Computed:    true,
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var dbPlan projectDatabaseModel
	resp.Diagnostics.Append(plan.Database.As(ctx, &dbPlan, objectAsOptions())...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := pgbeam.CreateProjectRequest{
		Name:  plan.Name.ValueString(),
		OrgID: plan.OrgID.ValueString(),
		Database: pgbeam.CreateDatabaseRequest{
			Host:     dbPlan.Host.ValueString(),
			Port:     int(dbPlan.Port.ValueInt64()),
			Name:     dbPlan.Name.ValueString(),
			Username: dbPlan.Username.ValueString(),
			Password: dbPlan.Password.ValueString(),
		},
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		desc := plan.Description.ValueString()
		createReq.Description = &desc
	}

	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		var tags []string
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Tags = tags
	}

	if !plan.Cloud.IsNull() && !plan.Cloud.IsUnknown() {
		createReq.Cloud = plan.Cloud.ValueString()
	}

	if !dbPlan.SSLMode.IsNull() && !dbPlan.SSLMode.IsUnknown() {
		createReq.Database.SSLMode = dbPlan.SSLMode.ValueString()
	}
	if !dbPlan.Role.IsNull() && !dbPlan.Role.IsUnknown() {
		createReq.Database.Role = dbPlan.Role.ValueString()
	}
	if !dbPlan.PoolRegion.IsNull() && !dbPlan.PoolRegion.IsUnknown() {
		pr := dbPlan.PoolRegion.ValueString()
		createReq.Database.PoolRegion = &pr
	}

	if !dbPlan.CacheConfig.IsNull() && !dbPlan.CacheConfig.IsUnknown() {
		var cc cacheConfigModel
		resp.Diagnostics.Append(dbPlan.CacheConfig.As(ctx, &cc, objectAsOptions())...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Database.CacheConfig = &pgbeam.CacheConfig{
			Enabled:    cc.Enabled.ValueBool(),
			TTLSeconds: int(cc.TTLSeconds.ValueInt64()),
			MaxEntries: int(cc.MaxEntries.ValueInt64()),
			SWRSeconds: int(cc.SWRSeconds.ValueInt64()),
		}
	}

	if !dbPlan.PoolConfig.IsNull() && !dbPlan.PoolConfig.IsUnknown() {
		var pc poolConfigModel
		resp.Diagnostics.Append(dbPlan.PoolConfig.As(ctx, &pc, objectAsOptions())...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Database.PoolConfig = &pgbeam.PoolConfig{
			PoolSize:    int(pc.PoolSize.ValueInt64()),
			MinPoolSize: int(pc.MinPoolSize.ValueInt64()),
			PoolMode:    pc.PoolMode.ValueString(),
		}
	}

	result, err := r.client.Projects.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	// Rate-limit fields are not supported in CreateProjectRequest.
	// If specified, issue an immediate update after creation.
	updateReq := pgbeam.UpdateProjectRequest{}
	needsPostCreateUpdate := false
	if !plan.QueriesPerSecond.IsNull() && !plan.QueriesPerSecond.IsUnknown() {
		qps := int32(plan.QueriesPerSecond.ValueInt64())
		updateReq.QueriesPerSecond = &qps
		needsPostCreateUpdate = true
	}
	if !plan.BurstSize.IsNull() && !plan.BurstSize.IsUnknown() {
		bs := int32(plan.BurstSize.ValueInt64())
		updateReq.BurstSize = &bs
		needsPostCreateUpdate = true
	}
	if !plan.MaxConnections.IsNull() && !plan.MaxConnections.IsUnknown() {
		mc := int32(plan.MaxConnections.ValueInt64())
		updateReq.MaxConnections = &mc
		needsPostCreateUpdate = true
	}
	if needsPostCreateUpdate {
		if _, err := r.client.Projects.Update(ctx, result.Project.ID, updateReq); err != nil {
			resp.Diagnostics.AddError("Error setting rate limits on newly created project", err.Error())
			return
		}
		// Re-read to get updated values
		updated, err := r.client.Projects.Get(ctx, result.Project.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading project after rate limit update", err.Error())
			return
		}
		result.Project = *updated
	}

	plan.ID = types.StringValue(result.Project.ID)
	r.mapProjectToState(ctx, &plan, &result.Project, &resp.Diagnostics)

	if result.Database != nil {
		plan.PrimaryDatabaseID = types.StringValue(result.Database.ID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.Projects.Get(ctx, state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	r.mapProjectToState(ctx, &state, project, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := pgbeam.UpdateProjectRequest{}
	hasChanges := false

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
		hasChanges = true
	}

	if !plan.Description.Equal(state.Description) {
		if plan.Description.IsNull() {
			empty := ""
			updateReq.Description = &empty
		} else {
			desc := plan.Description.ValueString()
			updateReq.Description = &desc
		}
		hasChanges = true
	}

	if !plan.Tags.Equal(state.Tags) {
		var tags []string
		if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
			resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		updateReq.Tags = &tags
		hasChanges = true
	}

	if !plan.QueriesPerSecond.Equal(state.QueriesPerSecond) {
		qps := int32(plan.QueriesPerSecond.ValueInt64())
		updateReq.QueriesPerSecond = &qps
		hasChanges = true
	}

	if !plan.BurstSize.Equal(state.BurstSize) {
		bs := int32(plan.BurstSize.ValueInt64())
		updateReq.BurstSize = &bs
		hasChanges = true
	}

	if !plan.MaxConnections.Equal(state.MaxConnections) {
		mc := int32(plan.MaxConnections.ValueInt64())
		updateReq.MaxConnections = &mc
		hasChanges = true
	}

	if hasChanges {
		_, err := r.client.Projects.Update(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating project", err.Error())
			return
		}
	}

	project, err := r.client.Projects.Get(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading project after update", err.Error())
		return
	}

	plan.ID = state.ID
	plan.PrimaryDatabaseID = state.PrimaryDatabaseID
	// Preserve the database block from state — the project Update API does not
	// modify the primary database. Users should use pgbeam_database for that.
	plan.Database = state.Database
	r.mapProjectToState(ctx, &plan, project, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Projects.Delete(ctx, state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, err := r.client.Projects.Get(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing project", err.Error())
		return
	}

	var state projectResourceModel
	state.ID = types.StringValue(project.ID)
	// Database block cannot be populated from read — set as null
	state.Database = types.ObjectNull(projectDatabaseAttrTypes())
	r.mapProjectToState(ctx, &state, project, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) mapProjectToState(ctx context.Context, state *projectResourceModel, p *pgbeam.Project, diags *diag.Diagnostics) {
	state.OrgID = types.StringValue(p.OrgID)
	state.Name = types.StringValue(p.Name)

	if p.Description != nil {
		state.Description = types.StringValue(*p.Description)
	} else if state.Description.IsNull() {
		// keep null
	} else {
		state.Description = types.StringNull()
	}

	if len(p.Tags) > 0 {
		tagValues := make([]attr.Value, len(p.Tags))
		for i, t := range p.Tags {
			tagValues[i] = types.StringValue(t)
		}
		tagsList, d := types.ListValue(types.StringType, tagValues)
		diags.Append(d...)
		state.Tags = tagsList
	} else if !state.Tags.IsNull() {
		state.Tags = types.ListNull(types.StringType)
	}

	if p.Cloud != "" {
		state.Cloud = types.StringValue(p.Cloud)
	} else {
		state.Cloud = types.StringValue("aws")
	}

	state.ProxyHost = types.StringValue(p.ProxyHost)
	state.QueriesPerSecond = types.Int64Value(int64(p.QueriesPerSecond))
	state.BurstSize = types.Int64Value(int64(p.BurstSize))
	state.MaxConnections = types.Int64Value(int64(p.MaxConnections))
	state.DatabaseCount = types.Int64Value(int64(p.DatabaseCount))
	state.ActiveConnections = types.Int64Value(int64(p.ActiveConnections))
	state.Status = types.StringValue(p.Status)
	state.CreatedAt = types.StringValue(p.CreatedAt)
	state.UpdatedAt = types.StringValue(p.UpdatedAt)
}
