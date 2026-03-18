package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pgbeam "github.com/pgbeam/pgbeam-go"
)

var (
	_ resource.Resource                = (*databaseResource)(nil)
	_ resource.ResourceWithConfigure   = (*databaseResource)(nil)
	_ resource.ResourceWithImportState = (*databaseResource)(nil)
)

type databaseResource struct {
	client *pgbeam.Client
}

type databaseResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectID        types.String `tfsdk:"project_id"`
	Host             types.String `tfsdk:"host"`
	Port             types.Int64  `tfsdk:"port"`
	Name             types.String `tfsdk:"name"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	SSLMode          types.String `tfsdk:"ssl_mode"`
	Role             types.String `tfsdk:"role"`
	PoolRegion       types.String `tfsdk:"pool_region"`
	CacheConfig      types.Object `tfsdk:"cache_config"`
	PoolConfig       types.Object `tfsdk:"pool_config"`
	ConnectionString types.String `tfsdk:"connection_string"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func NewDatabaseResource() resource.Resource {
	return &databaseResource{}
}

func (r *databaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (r *databaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an upstream database connection within a PgBeam project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Database ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID this database belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
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
			"connection_string": schema.StringAttribute{
				Description: "Connection string for this database via PgBeam proxy.",
				Computed:    true,
				Sensitive:   true,
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

func (r *databaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *databaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := pgbeam.CreateDatabaseRequest{
		Host:     plan.Host.ValueString(),
		Port:     int(plan.Port.ValueInt64()),
		Name:     plan.Name.ValueString(),
		Username: plan.Username.ValueString(),
		Password: plan.Password.ValueString(),
	}

	if !plan.SSLMode.IsNull() && !plan.SSLMode.IsUnknown() {
		createReq.SSLMode = plan.SSLMode.ValueString()
	}
	if !plan.Role.IsNull() && !plan.Role.IsUnknown() {
		createReq.Role = plan.Role.ValueString()
	}
	if !plan.PoolRegion.IsNull() && !plan.PoolRegion.IsUnknown() {
		pr := plan.PoolRegion.ValueString()
		createReq.PoolRegion = &pr
	}

	if !plan.CacheConfig.IsNull() && !plan.CacheConfig.IsUnknown() {
		var cc cacheConfigModel
		resp.Diagnostics.Append(plan.CacheConfig.As(ctx, &cc, objectAsOptions())...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.CacheConfig = &pgbeam.CacheConfig{
			Enabled:    cc.Enabled.ValueBool(),
			TTLSeconds: int(cc.TTLSeconds.ValueInt64()),
			MaxEntries: int(cc.MaxEntries.ValueInt64()),
			SWRSeconds: int(cc.SWRSeconds.ValueInt64()),
		}
	}

	if !plan.PoolConfig.IsNull() && !plan.PoolConfig.IsUnknown() {
		var pc poolConfigModel
		resp.Diagnostics.Append(plan.PoolConfig.As(ctx, &pc, objectAsOptions())...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.PoolConfig = &pgbeam.PoolConfig{
			PoolSize:    int(pc.PoolSize.ValueInt64()),
			MinPoolSize: int(pc.MinPoolSize.ValueInt64()),
			PoolMode:    pc.PoolMode.ValueString(),
		}
	}

	db, err := r.client.Databases.Create(ctx, plan.ProjectID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating database", err.Error())
		return
	}

	plan.ID = types.StringValue(db.ID)
	r.mapDatabaseToState(ctx, &plan, db, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	db, err := r.client.Databases.Get(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading database", err.Error())
		return
	}

	r.mapDatabaseToState(ctx, &state, db, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan databaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state databaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := pgbeam.UpdateDatabaseRequest{}
	hasChanges := false

	if !plan.Host.Equal(state.Host) {
		h := plan.Host.ValueString()
		updateReq.Host = &h
		hasChanges = true
	}
	if !plan.Port.Equal(state.Port) {
		p := int(plan.Port.ValueInt64())
		updateReq.Port = &p
		hasChanges = true
	}
	if !plan.Name.Equal(state.Name) {
		n := plan.Name.ValueString()
		updateReq.Name = &n
		hasChanges = true
	}
	if !plan.Username.Equal(state.Username) {
		u := plan.Username.ValueString()
		updateReq.Username = &u
		hasChanges = true
	}
	if !plan.Password.Equal(state.Password) {
		pw := plan.Password.ValueString()
		updateReq.Password = &pw
		hasChanges = true
	}
	if !plan.SSLMode.Equal(state.SSLMode) {
		s := plan.SSLMode.ValueString()
		updateReq.SSLMode = &s
		hasChanges = true
	}
	if !plan.Role.Equal(state.Role) {
		role := plan.Role.ValueString()
		updateReq.Role = &role
		hasChanges = true
	}
	if !plan.PoolRegion.Equal(state.PoolRegion) {
		if plan.PoolRegion.IsNull() {
			empty := ""
			updateReq.PoolRegion = &empty
		} else {
			pr := plan.PoolRegion.ValueString()
			updateReq.PoolRegion = &pr
		}
		hasChanges = true
	}

	if !plan.CacheConfig.Equal(state.CacheConfig) {
		if !plan.CacheConfig.IsNull() && !plan.CacheConfig.IsUnknown() {
			var cc cacheConfigModel
			resp.Diagnostics.Append(plan.CacheConfig.As(ctx, &cc, objectAsOptions())...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.CacheConfig = &pgbeam.CacheConfig{
				Enabled:    cc.Enabled.ValueBool(),
				TTLSeconds: int(cc.TTLSeconds.ValueInt64()),
				MaxEntries: int(cc.MaxEntries.ValueInt64()),
				SWRSeconds: int(cc.SWRSeconds.ValueInt64()),
			}
			hasChanges = true
		}
	}

	if !plan.PoolConfig.Equal(state.PoolConfig) {
		if !plan.PoolConfig.IsNull() && !plan.PoolConfig.IsUnknown() {
			var pc poolConfigModel
			resp.Diagnostics.Append(plan.PoolConfig.As(ctx, &pc, objectAsOptions())...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.PoolConfig = &pgbeam.PoolConfig{
				PoolSize:    int(pc.PoolSize.ValueInt64()),
				MinPoolSize: int(pc.MinPoolSize.ValueInt64()),
				PoolMode:    pc.PoolMode.ValueString(),
			}
			hasChanges = true
		}
	}

	if hasChanges {
		_, err := r.client.Databases.Update(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating database", err.Error())
			return
		}
	}

	db, err := r.client.Databases.Get(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading database after update", err.Error())
		return
	}

	plan.ID = state.ID
	r.mapDatabaseToState(ctx, &plan, db, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Databases.Delete(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting database", err.Error())
	}
}

func (r *databaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Expected format: project_id/database_id")
		return
	}

	projectID := parts[0]
	databaseID := parts[1]

	db, err := r.client.Databases.Get(ctx, projectID, databaseID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing database", err.Error())
		return
	}

	var state databaseResourceModel
	state.ID = types.StringValue(db.ID)
	state.ProjectID = types.StringValue(projectID)
	// Password cannot be read back from API
	state.Password = types.StringValue("")
	r.mapDatabaseToState(ctx, &state, db, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseResource) mapDatabaseToState(ctx context.Context, state *databaseResourceModel, db *pgbeam.Database, diags *diag.Diagnostics) {
	state.ProjectID = types.StringValue(db.ProjectID)
	state.Host = types.StringValue(db.Host)
	state.Port = types.Int64Value(int64(db.Port))
	state.Name = types.StringValue(db.Name)
	state.Username = types.StringValue(db.Username)

	if db.SSLMode != "" {
		state.SSLMode = types.StringValue(db.SSLMode)
	} else {
		state.SSLMode = types.StringNull()
	}

	if db.Role != "" {
		state.Role = types.StringValue(db.Role)
	} else {
		state.Role = types.StringNull()
	}

	if db.PoolRegion != nil {
		state.PoolRegion = types.StringValue(*db.PoolRegion)
	} else {
		state.PoolRegion = types.StringNull()
	}

	ccObj, d := types.ObjectValue(cacheConfigAttrTypes(), map[string]attr.Value{
		"enabled":     types.BoolValue(db.CacheConfig.Enabled),
		"ttl_seconds": types.Int64Value(int64(db.CacheConfig.TTLSeconds)),
		"max_entries": types.Int64Value(int64(db.CacheConfig.MaxEntries)),
		"swr_seconds": types.Int64Value(int64(db.CacheConfig.SWRSeconds)),
	})
	diags.Append(d...)
	state.CacheConfig = ccObj

	pcObj, d := types.ObjectValue(poolConfigAttrTypes(), map[string]attr.Value{
		"pool_size":     types.Int64Value(int64(db.PoolConfig.PoolSize)),
		"min_pool_size": types.Int64Value(int64(db.PoolConfig.MinPoolSize)),
		"pool_mode":     types.StringValue(db.PoolConfig.PoolMode),
	})
	diags.Append(d...)
	state.PoolConfig = pcObj

	if db.ConnectionString != nil {
		state.ConnectionString = types.StringValue(*db.ConnectionString)
	} else {
		state.ConnectionString = types.StringNull()
	}

	state.CreatedAt = types.StringValue(db.CreatedAt)
	state.UpdatedAt = types.StringValue(db.UpdatedAt)
}
