package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type trapDataSource struct{ client *Client }

type trapModel struct {
	TrapURL   types.String `tfsdk:"trap_url"`
	APIURL    types.String `tfsdk:"api_url"`
	IngestURL types.String `tfsdk:"ingest_url"`
}

func newTrapDataSource() datasource.DataSource { return &trapDataSource{} }

func (d *trapDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trap"
}

func (d *trapDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Where decoys point and where audit forwarders send events.",
		Attributes: map[string]schema.Attribute{
			"trap_url":   schema.StringAttribute{Computed: true, Description: "Base URL decoy credentials point at."},
			"api_url":    schema.StringAttribute{Computed: true, Description: "Lilytrap API."},
			"ingest_url": schema.StringAttribute{Computed: true, Description: "Base URL for audit forwarders: append aws, gcp, azure or host."},
		},
	}
}

func (d *trapDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*Client); ok {
		d.client = c
	}
}

func (d *trapDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	trapURL, err := d.client.TrapURL(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Couldn't reach Lilytrap", err.Error())
		return
	}
	api := trimSlash(d.client.APIURL)
	resp.Diagnostics.Append(resp.State.Set(ctx, trapModel{
		TrapURL:   types.StringValue(trapURL),
		APIURL:    types.StringValue(api),
		IngestURL: types.StringValue(api + "/ingest/v1"),
	})...)
}
