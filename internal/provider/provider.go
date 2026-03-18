package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pgbeam "github.com/pgbeam/pgbeam-go"
)

var _ provider.Provider = (*pgbeamProvider)(nil)

type pgbeamProvider struct {
	version string
}

type pgbeamProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &pgbeamProvider{version: version}
	}
}

func (p *pgbeamProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pgbeam"
	resp.Version = p.version
}

func (p *pgbeamProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The PgBeam provider configures access to the PgBeam Control Plane API for managing globally distributed PostgreSQL proxy infrastructure.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "PgBeam API key for authentication. Can also be set via the PGBEAM_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Base URL for the PgBeam API. Defaults to https://api.pgbeam.com. Can also be set via the PGBEAM_API_URL environment variable.",
				Optional:    true,
			},
		},
	}
}

func (p *pgbeamProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config pgbeamProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("PGBEAM_API_KEY")
	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() {
		apiKey = config.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The PgBeam API key is required. Set it in the provider configuration or via the PGBEAM_API_KEY environment variable.",
		)
		return
	}

	baseURL := os.Getenv("PGBEAM_API_URL")
	if !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() {
		baseURL = config.BaseURL.ValueString()
	}

	client := pgbeam.NewClient(&pgbeam.ClientOptions{
		APIKey:  apiKey,
		BaseURL: baseURL,
	})

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *pgbeamProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewDatabaseResource,
		NewReplicaResource,
		NewCustomDomainResource,
		NewCacheRuleResource,
		NewSpendLimitResource,
	}
}

func (p *pgbeamProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
