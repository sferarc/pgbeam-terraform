package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pgbeam "github.com/pgbeam/pgbeam-go"
)

var (
	_ resource.Resource                = (*replicaResource)(nil)
	_ resource.ResourceWithConfigure   = (*replicaResource)(nil)
	_ resource.ResourceWithImportState = (*replicaResource)(nil)
)

type replicaResource struct {
	client *pgbeam.Client
}

type replicaResourceModel struct {
	ID         types.String `tfsdk:"id"`
	DatabaseID types.String `tfsdk:"database_id"`
	Host       types.String `tfsdk:"host"`
	Port       types.Int64  `tfsdk:"port"`
	SSLMode    types.String `tfsdk:"ssl_mode"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

func NewReplicaResource() resource.Resource {
	return &replicaResource{}
}

func (r *replicaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_replica"
}

func (r *replicaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a read replica endpoint for a PgBeam database. This resource is immutable — any change to inputs triggers a delete and recreate.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Replica ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"database_id": schema.StringAttribute{
				Description: "Database ID this replica belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				Description: "Read replica host.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"port": schema.Int64Attribute{
				Description: "Read replica port (1-65535).",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"ssl_mode": schema.StringAttribute{
				Description: "SSL connection mode: disable, allow, prefer, require, verify-ca, or verify-full.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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

func (r *replicaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *replicaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan replicaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := pgbeam.CreateReplicaRequest{
		Host: plan.Host.ValueString(),
		Port: int(plan.Port.ValueInt64()),
	}

	if !plan.SSLMode.IsNull() && !plan.SSLMode.IsUnknown() {
		createReq.SSLMode = plan.SSLMode.ValueString()
	}

	replica, err := r.client.Replicas.Create(ctx, plan.DatabaseID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating replica", err.Error())
		return
	}

	plan.ID = types.StringValue(replica.ID)
	plan.DatabaseID = types.StringValue(replica.DatabaseID)
	plan.Host = types.StringValue(replica.Host)
	plan.Port = types.Int64Value(int64(replica.Port))
	if replica.SSLMode != "" {
		plan.SSLMode = types.StringValue(replica.SSLMode)
	} else {
		plan.SSLMode = types.StringNull()
	}
	plan.CreatedAt = types.StringValue(replica.CreatedAt)
	plan.UpdatedAt = types.StringValue(replica.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *replicaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state replicaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	replica, err := r.client.Replicas.Get(ctx, state.DatabaseID.ValueString(), state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading replica", err.Error())
		return
	}

	state.Host = types.StringValue(replica.Host)
	state.Port = types.Int64Value(int64(replica.Port))
	if replica.SSLMode != "" {
		state.SSLMode = types.StringValue(replica.SSLMode)
	} else {
		state.SSLMode = types.StringNull()
	}
	state.CreatedAt = types.StringValue(replica.CreatedAt)
	state.UpdatedAt = types.StringValue(replica.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not implemented — all fields RequiresReplace, so Terraform will
// never call Update. It is included to satisfy the Resource interface.
func (r *replicaResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported", "Replica resources are immutable. All changes trigger a replacement.")
}

func (r *replicaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state replicaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Replicas.Delete(ctx, state.DatabaseID.ValueString(), state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting replica", err.Error())
	}
}

func (r *replicaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Expected format: database_id/replica_id")
		return
	}

	databaseID := parts[0]
	replicaID := parts[1]

	replica, err := r.client.Replicas.Get(ctx, databaseID, replicaID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing replica", err.Error())
		return
	}

	var state replicaResourceModel
	state.ID = types.StringValue(replica.ID)
	state.DatabaseID = types.StringValue(databaseID)
	state.Host = types.StringValue(replica.Host)
	state.Port = types.Int64Value(int64(replica.Port))
	if replica.SSLMode != "" {
		state.SSLMode = types.StringValue(replica.SSLMode)
	} else {
		state.SSLMode = types.StringNull()
	}
	state.CreatedAt = types.StringValue(replica.CreatedAt)
	state.UpdatedAt = types.StringValue(replica.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
