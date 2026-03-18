package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource                = (*spendLimitResource)(nil)
	_ resource.ResourceWithConfigure   = (*spendLimitResource)(nil)
	_ resource.ResourceWithImportState = (*spendLimitResource)(nil)
)

type spendLimitResource struct {
	client *pgbeam.Client
}

type spendLimitResourceModel struct {
	ID                 types.String  `tfsdk:"id"`
	OrgID              types.String  `tfsdk:"org_id"`
	SpendLimit         types.Float64 `tfsdk:"spend_limit"`
	Plan               types.String  `tfsdk:"plan"`
	BillingProvider    types.String  `tfsdk:"billing_provider"`
	SubscriptionStatus types.String  `tfsdk:"subscription_status"`
	CurrentPeriodEnd   types.String  `tfsdk:"current_period_end"`
	Enabled            types.Bool    `tfsdk:"enabled"`
	CustomPricing      types.Bool    `tfsdk:"custom_pricing"`
	Limits             types.Object  `tfsdk:"limits"`
}

func planLimitsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"queries_per_day":    types.Int64Type,
		"max_projects":       types.Int64Type,
		"max_databases":      types.Int64Type,
		"max_connections":    types.Int64Type,
		"queries_per_second": types.Int64Type,
		"bytes_per_month":    types.Int64Type,
		"max_query_shapes":   types.Int64Type,
		"included_seats":     types.Int64Type,
	}
}

func NewSpendLimitResource() resource.Resource {
	return &spendLimitResource{}
}

func (r *spendLimitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spend_limit"
}

func (r *spendLimitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the spend limit for a PgBeam organization. Uses PUT to create and update. Deletion removes the spend limit (sets to null).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Organization ID (same as org_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org_id": schema.StringAttribute{
				Description: "Organization ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"spend_limit": schema.Float64Attribute{
				Description: "Monthly spend limit in USD. Set to null to remove the limit.",
				Optional:    true,
			},
			"plan": schema.StringAttribute{
				Description: "Current billing plan name.",
				Computed:    true,
			},
			"billing_provider": schema.StringAttribute{
				Description: "Billing provider (e.g. stripe).",
				Computed:    true,
			},
			"subscription_status": schema.StringAttribute{
				Description: "Subscription status (e.g. active, canceled).",
				Computed:    true,
			},
			"current_period_end": schema.StringAttribute{
				Description: "ISO 8601 timestamp of when the current billing period ends.",
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the organization is enabled.",
				Computed:    true,
			},
			"custom_pricing": schema.BoolAttribute{
				Description: "Whether the organization has custom pricing.",
				Computed:    true,
			},
			"limits": schema.SingleNestedAttribute{
				Description: "Plan limits for this organization.",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"queries_per_day": schema.Int64Attribute{
						Description: "Maximum queries per day.",
						Computed:    true,
					},
					"max_projects": schema.Int64Attribute{
						Description: "Maximum number of projects.",
						Computed:    true,
					},
					"max_databases": schema.Int64Attribute{
						Description: "Maximum number of databases.",
						Computed:    true,
					},
					"max_connections": schema.Int64Attribute{
						Description: "Maximum concurrent connections.",
						Computed:    true,
					},
					"queries_per_second": schema.Int64Attribute{
						Description: "Maximum queries per second.",
						Computed:    true,
					},
					"bytes_per_month": schema.Int64Attribute{
						Description: "Maximum bytes per month.",
						Computed:    true,
					},
					"max_query_shapes": schema.Int64Attribute{
						Description: "Maximum number of unique query shapes.",
						Computed:    true,
					},
					"included_seats": schema.Int64Attribute{
						Description: "Number of included team seats.",
						Computed:    true,
					},
				},
			},
		},
	}
}

func (r *spendLimitResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *spendLimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan spendLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgID := plan.OrgID.ValueString()

	updateReq := pgbeam.UpdateSpendLimitRequest{}
	if !plan.SpendLimit.IsNull() && !plan.SpendLimit.IsUnknown() {
		sl := plan.SpendLimit.ValueFloat64()
		updateReq.SpendLimit = &sl
	}

	orgPlan, err := r.client.Analytics.UpdateSpendLimit(ctx, orgID, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating spend limit", err.Error())
		return
	}

	plan.ID = types.StringValue(orgID)
	r.mapOrgPlanToState(&plan, orgPlan, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *spendLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state spendLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgPlan, err := r.client.Analytics.GetOrganizationPlan(ctx, state.OrgID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading spend limit", err.Error())
		return
	}

	r.mapOrgPlanToState(&state, orgPlan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *spendLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan spendLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := pgbeam.UpdateSpendLimitRequest{}
	if !plan.SpendLimit.IsNull() && !plan.SpendLimit.IsUnknown() {
		sl := plan.SpendLimit.ValueFloat64()
		updateReq.SpendLimit = &sl
	}

	orgPlan, err := r.client.Analytics.UpdateSpendLimit(ctx, plan.OrgID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating spend limit", err.Error())
		return
	}

	r.mapOrgPlanToState(&plan, orgPlan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *spendLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state spendLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Analytics.RemoveSpendLimit(ctx, state.OrgID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting spend limit", err.Error())
	}
}

func (r *spendLimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	orgID := req.ID

	orgPlan, err := r.client.Analytics.GetOrganizationPlan(ctx, orgID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing spend limit", err.Error())
		return
	}

	var state spendLimitResourceModel
	state.ID = types.StringValue(orgID)
	state.OrgID = types.StringValue(orgID)
	r.mapOrgPlanToState(&state, orgPlan, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *spendLimitResource) mapOrgPlanToState(state *spendLimitResourceModel, op *pgbeam.OrganizationPlan, diags *diag.Diagnostics) {
	if op.SpendLimit != nil {
		state.SpendLimit = types.Float64Value(*op.SpendLimit)
	} else {
		state.SpendLimit = types.Float64Null()
	}

	state.Plan = types.StringValue(op.Plan)

	if op.BillingProvider != nil {
		state.BillingProvider = types.StringValue(*op.BillingProvider)
	} else {
		state.BillingProvider = types.StringNull()
	}

	if op.SubscriptionStatus != nil {
		state.SubscriptionStatus = types.StringValue(*op.SubscriptionStatus)
	} else {
		state.SubscriptionStatus = types.StringNull()
	}

	if op.CurrentPeriodEnd != nil {
		state.CurrentPeriodEnd = types.StringValue(op.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		state.CurrentPeriodEnd = types.StringNull()
	}

	if op.Enabled != nil {
		state.Enabled = types.BoolValue(*op.Enabled)
	} else {
		state.Enabled = types.BoolNull()
	}

	if op.CustomPricing != nil {
		state.CustomPricing = types.BoolValue(*op.CustomPricing)
	} else {
		state.CustomPricing = types.BoolNull()
	}

	limitsObj, d := types.ObjectValue(planLimitsAttrTypes(), map[string]attr.Value{
		"queries_per_day":    types.Int64Value(op.Limits.QueriesPerDay),
		"max_projects":       types.Int64Value(int64(op.Limits.MaxProjects)),
		"max_databases":      types.Int64Value(int64(op.Limits.MaxDatabases)),
		"max_connections":    types.Int64Value(int64(op.Limits.MaxConnections)),
		"queries_per_second": types.Int64Value(int64(op.Limits.QueriesPerSecond)),
		"bytes_per_month":    types.Int64Value(op.Limits.BytesPerMonth),
		"max_query_shapes":   types.Int64Value(int64(op.Limits.MaxQueryShapes)),
		"included_seats":     types.Int64Value(int64(op.Limits.IncludedSeats)),
	})
	diags.Append(d...)
	state.Limits = limitsObj
}
