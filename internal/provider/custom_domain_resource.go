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
	_ resource.Resource                = (*customDomainResource)(nil)
	_ resource.ResourceWithConfigure   = (*customDomainResource)(nil)
	_ resource.ResourceWithImportState = (*customDomainResource)(nil)
)

type customDomainResource struct {
	client *pgbeam.Client
}

type customDomainResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectID            types.String `tfsdk:"project_id"`
	Domain               types.String `tfsdk:"domain"`
	Verified             types.Bool   `tfsdk:"verified"`
	VerifiedAt           types.String `tfsdk:"verified_at"`
	TLSCertExpiry        types.String `tfsdk:"tls_cert_expiry"`
	DNSVerificationToken types.String `tfsdk:"dns_verification_token"`
	DNSInstructions      types.Object `tfsdk:"dns_instructions"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func dnsInstructionsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"cname_host":        types.StringType,
		"cname_target":      types.StringType,
		"txt_host":          types.StringType,
		"txt_value":         types.StringType,
		"acme_cname_host":   types.StringType,
		"acme_cname_target": types.StringType,
	}
}

func NewCustomDomainResource() resource.Resource {
	return &customDomainResource{}
}

func (r *customDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_domain"
}

func (r *customDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a custom domain attached to a PgBeam project. Custom domains are immutable — changing project_id or domain triggers a replacement.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Custom domain ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID this custom domain belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				Description: "The custom domain name (e.g. db.example.com).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"verified": schema.BoolAttribute{
				Description: "Whether the domain has been verified via DNS.",
				Computed:    true,
			},
			"verified_at": schema.StringAttribute{
				Description: "ISO 8601 timestamp of when the domain was verified.",
				Computed:    true,
			},
			"tls_cert_expiry": schema.StringAttribute{
				Description: "ISO 8601 timestamp of when the TLS certificate expires.",
				Computed:    true,
			},
			"dns_verification_token": schema.StringAttribute{
				Description: "Token used for DNS verification.",
				Computed:    true,
			},
			"dns_instructions": schema.SingleNestedAttribute{
				Description: "DNS records required for domain verification and TLS provisioning.",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"cname_host": schema.StringAttribute{
						Description: "CNAME record host.",
						Computed:    true,
					},
					"cname_target": schema.StringAttribute{
						Description: "CNAME record target.",
						Computed:    true,
					},
					"txt_host": schema.StringAttribute{
						Description: "TXT record host.",
						Computed:    true,
					},
					"txt_value": schema.StringAttribute{
						Description: "TXT record value.",
						Computed:    true,
					},
					"acme_cname_host": schema.StringAttribute{
						Description: "ACME CNAME record host for TLS certificate validation.",
						Computed:    true,
					},
					"acme_cname_target": schema.StringAttribute{
						Description: "ACME CNAME record target for TLS certificate validation.",
						Computed:    true,
					},
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

func (r *customDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *customDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := pgbeam.CreateCustomDomainRequest{
		Domain: plan.Domain.ValueString(),
	}

	domain, err := r.client.Domains.Create(ctx, plan.ProjectID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating custom domain", err.Error())
		return
	}

	plan.ID = types.StringValue(domain.ID)
	r.mapCustomDomainToState(&plan, domain, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.Domains.Get(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading custom domain", err.Error())
		return
	}

	r.mapCustomDomainToState(&state, domain, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not implemented — project_id and domain both RequiresReplace,
// so Terraform will never call Update.
func (r *customDomainResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported", "Custom domain resources are immutable. All changes trigger a replacement.")
}

func (r *customDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Domains.Delete(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting custom domain", err.Error())
	}
}

func (r *customDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Expected format: project_id/domain_id")
		return
	}

	projectID := parts[0]
	domainID := parts[1]

	domain, err := r.client.Domains.Get(ctx, projectID, domainID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing custom domain", err.Error())
		return
	}

	var state customDomainResourceModel
	state.ID = types.StringValue(domain.ID)
	state.ProjectID = types.StringValue(projectID)
	r.mapCustomDomainToState(&state, domain, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customDomainResource) mapCustomDomainToState(state *customDomainResourceModel, d *pgbeam.CustomDomain, diags *diag.Diagnostics) {
	state.ProjectID = types.StringValue(d.ProjectID)
	state.Domain = types.StringValue(d.Domain)
	state.Verified = types.BoolValue(d.Verified)

	if d.VerifiedAt != nil {
		state.VerifiedAt = types.StringValue(d.VerifiedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		state.VerifiedAt = types.StringNull()
	}

	if d.TLSCertExpiry != nil {
		state.TLSCertExpiry = types.StringValue(d.TLSCertExpiry.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		state.TLSCertExpiry = types.StringNull()
	}

	state.DNSVerificationToken = types.StringValue(d.DNSVerificationToken)

	if d.DNSInstructions != nil {
		dnsObj, dd := types.ObjectValue(dnsInstructionsAttrTypes(), map[string]attr.Value{
			"cname_host":        types.StringValue(d.DNSInstructions.CNAMEHost),
			"cname_target":      types.StringValue(d.DNSInstructions.CNAMETarget),
			"txt_host":          types.StringValue(d.DNSInstructions.TXTHost),
			"txt_value":         types.StringValue(d.DNSInstructions.TXTValue),
			"acme_cname_host":   types.StringValue(d.DNSInstructions.ACMECNAMEHost),
			"acme_cname_target": types.StringValue(d.DNSInstructions.ACMECNAMETarget),
		})
		diags.Append(dd...)
		state.DNSInstructions = dnsObj
	} else {
		state.DNSInstructions = types.ObjectNull(dnsInstructionsAttrTypes())
	}

	state.CreatedAt = types.StringValue(d.CreatedAt)
	state.UpdatedAt = types.StringValue(d.UpdatedAt)
}
