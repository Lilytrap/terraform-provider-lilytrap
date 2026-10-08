package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// policyDataSource asks which of the paths a configuration is about to put decoys at are excluded by
// the workspace's ignore rules (plus its own), so they can be skipped with count or for_each.
type policyDataSource struct{ client *Client }

type policyModel struct {
	Paths   types.List `tfsdk:"paths"`
	Ignore  types.List `tfsdk:"ignore"`
	Ignored types.List `tfsdk:"ignored"`
	Allowed types.List `tfsdk:"allowed"`
}

func newPolicyDataSource() datasource.DataSource { return &policyDataSource{} }

func (d *policyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *policyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Which decoy locations Lilytrap's ignore rules exclude. Paths use the .lilyignore resource syntax, " +
			"e.g. aws/<account>/<region>/secretsmanager/<name>, vault/<mount>/<path>, k8s/<namespace>/secrets/<name>. " +
			"Use `ignored` or `allowed` in count/for_each to skip excluded decoys.",
		Attributes: map[string]schema.Attribute{
			"paths": schema.ListAttribute{
				ElementType: types.StringType, Required: true,
				Validators:  []validator.List{listvalidator.SizeBetween(1, 1000)},
				Description: "Resource paths about to receive decoys.",
			},
			"ignore": schema.ListAttribute{
				ElementType: types.StringType, Optional: true,
				Validators:  []validator.List{listvalidator.SizeAtMost(200)},
				Description: "Extra .lilyignore patterns for this configuration. The workspace's rules always apply too.",
			},
			"ignored": schema.ListAttribute{ElementType: types.StringType, Computed: true, Description: "The paths the rules exclude."},
			"allowed": schema.ListAttribute{ElementType: types.StringType, Computed: true, Description: "The paths decoys may go to."},
		},
	}
}

func (d *policyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*Client); ok {
		d.client = c
	}
}

func (d *policyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg policyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client.APIKey == "" {
		resp.Diagnostics.AddError("Missing Lilytrap API key", "Set api_key in the provider block or LILYTRAP_API_KEY.")
		return
	}
	var paths, extra []string
	resp.Diagnostics.Append(cfg.Paths.ElementsAs(ctx, &paths, false)...)
	if !cfg.Ignore.IsNull() && !cfg.Ignore.IsUnknown() {
		resp.Diagnostics.Append(cfg.Ignore.ElementsAs(ctx, &extra, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	ignored, err := d.client.CheckPolicy(ctx, paths, extra)
	if errors.Is(err, ErrNotFound) {
		// An API from before ignore rules. Without local rules there's nothing to exclude.
		if len(extra) > 0 {
			resp.Diagnostics.AddError("Lilytrap can't check ignore rules", "This Lilytrap API predates ignore rules, so `ignore` can't be applied.")
			return
		}
		ignored = nil
	} else if err != nil {
		resp.Diagnostics.AddError("Couldn't read Lilytrap's ignore rules", err.Error())
		return
	}
	excluded := map[string]bool{}
	for _, p := range ignored {
		excluded[p] = true
	}
	allowed := []string{}
	for _, p := range paths {
		if !excluded[p] {
			allowed = append(allowed, p)
		}
	}
	if ignored == nil {
		ignored = []string{}
	}
	cfg.Ignored, _ = types.ListValueFrom(ctx, types.StringType, ignored)
	cfg.Allowed, _ = types.ListValueFrom(ctx, types.StringType, allowed)
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
