// Package provider implements the Lilytrap Terraform provider.
//
// Resources:
//   - lilytrap_decoy_credential: a decoy secret in Lilytrap's own format plus the trap URL it points at
//   - lilytrap_deployment: registers decoys (by hash) and retires them on destroy
//
// Data sources:
//   - lilytrap_trap: the trap and ingest URLs for this workspace
//   - lilytrap_policy: which of the given resource paths the workspace's ignore rules exclude
package provider

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultAPIURL = "https://api.lilytrap.com"

type lilytrapProvider struct{ version string }

type providerModel struct {
	APIURL types.String `tfsdk:"api_url"`
	APIKey types.String `tfsdk:"api_key"`
}

// New returns the provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &lilytrapProvider{version: version} }
}

func (p *lilytrapProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "lilytrap"
	resp.Version = p.version
}

func (p *lilytrapProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Plant decoy credentials in your infrastructure and get alerted when an agent finds or uses them.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Optional:    true,
				Description: "Lilytrap API. Defaults to LILYTRAP_API_URL, then " + defaultAPIURL + ".",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Workspace API key (wsk_...). Defaults to LILYTRAP_API_KEY. Only hashes of decoy secrets are sent with it.",
			},
		},
	}
}

func (p *lilytrapProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiURL := firstNonEmpty(cfg.APIURL.ValueString(), os.Getenv("LILYTRAP_API_URL"), defaultAPIURL)
	apiKey := firstNonEmpty(cfg.APIKey.ValueString(), os.Getenv("LILYTRAP_API_KEY"))
	if apiKey != "" && !strings.HasPrefix(apiKey, "wsk_") {
		resp.Diagnostics.AddError("Invalid Lilytrap API key", "The workspace API key starts with wsk_. Find it in the dashboard under Settings.")
		return
	}
	client := &Client{APIURL: apiURL, APIKey: apiKey, HTTP: &http.Client{Timeout: 30 * time.Second}, Version: p.version}
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *lilytrapProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{newCredentialResource, newDeploymentResource}
}

func (p *lilytrapProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{newTrapDataSource, newPolicyDataSource}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
