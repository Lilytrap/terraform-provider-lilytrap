package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type deploymentResource struct{ client *Client }

var _ resource.ResourceWithModifyPlan = (*deploymentResource)(nil)

type deploymentModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Target            types.String `tfsdk:"target"`
	Location          types.String `tfsdk:"location"`
	TrustedIdentities types.List   `tfsdk:"trusted_identities"`
	AccessDetection   types.Bool   `tfsdk:"access_detection"`
	Paths             types.List   `tfsdk:"paths"`
	Ignore            types.List   `tfsdk:"ignore"`
	Decoys            []decoyModel `tfsdk:"decoys"`
	CreatedAt         types.String `tfsdk:"created_at"`
	TrapURL           types.String `tfsdk:"trap_url"`
	IngestKey         types.String `tfsdk:"ingest_key"`
	IngestURL         types.String `tfsdk:"ingest_url"`
}

type decoyModel struct {
	Kit       types.String `tfsdk:"kit"`
	Secret    types.String `tfsdk:"secret"`
	Path      types.String `tfsdk:"path"`
	Method    types.String `tfsdk:"method"`
	Kind      types.String `tfsdk:"kind"`
	Hop       types.Int64  `tfsdk:"hop"`
	Resources types.List   `tfsdk:"resources"`
	Locations types.List   `tfsdk:"locations"`
}

func newDeploymentResource() resource.Resource { return &deploymentResource{} }

func (r *deploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (r *deploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	strList := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{ElementType: types.StringType, Optional: true, Description: desc}
	}
	resp.Schema = schema.Schema{
		Description: "Registers decoys with Lilytrap. Only sha256 hashes of the secrets are sent. " +
			"Any change creates a new deployment; destroying it retires the decoys, so they stop matching.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "Deployment id (bld_...)."},
			"name": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Shown in the dashboard, e.g. aws/prod.", Validators: []validator.String{stringvalidator.LengthBetween(1, 300)}},
			"target": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("cloud"), PlanModifiers: replace,
				Validators: []validator.String{stringvalidator.OneOf("cloud", "k8s", "host")},
			},
			"location": schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "Where the decoys live, e.g. aws:123456789012/us-east-2.", Validators: []validator.String{stringvalidator.LengthAtMost(300)}},
			"trusted_identities": schema.ListAttribute{
				ElementType: types.StringType, Optional: true,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
				Validators:    []validator.List{listvalidator.SizeAtMost(50)},
				Description:   "Identities that read these decoys as part of their job (the Terraform role, a backup job). Their reads never alert. IP, CIDR, ARN, glob, or exe:/path.",
			},
			"access_detection": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:   "Generate an ingest key so an audit-log forwarder can report reads of the decoys.",
			},
			"paths": schema.ListAttribute{
				ElementType: types.StringType, Optional: true,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
				Validators:    []validator.List{listvalidator.SizeAtMost(500)},
				Description: "Where these decoys live, as .lilyignore resource paths (e.g. vault/secret/prod/admin). " +
					"Checked against the workspace's ignore rules and `ignore` at plan time: an excluded path fails the plan before anything is created.",
			},
			"ignore": schema.ListAttribute{
				ElementType: types.StringType, Optional: true,
				Validators:  []validator.List{listvalidator.SizeAtMost(200)},
				Description: "Extra .lilyignore patterns for `paths`, on top of the workspace's rules.",
			},
			"decoys": schema.ListNestedAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
				Validators:    []validator.List{listvalidator.SizeBetween(1, 500)},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"kit":    schema.StringAttribute{Required: true, Description: "What the decoy pretends to be, e.g. cloud-secret.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9-]{1,40}$`), "lowercase letters, digits and dashes")}},
					"secret": schema.StringAttribute{Required: true, Sensitive: true, Description: "The decoy secret, usually lilytrap_decoy_credential.x.secret. Only its hash leaves Terraform."},
					"path":   schema.StringAttribute{Required: true, Description: "Path on the trap the secret is used against.", Validators: []validator.String{stringvalidator.LengthBetween(1, 512)}},
					"method": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("GET")},
					"kind": schema.StringAttribute{
						Optional: true, Computed: true, Default: stringdefault.StaticString("bearer"),
						Validators: []validator.String{stringvalidator.OneOf("bearer", "header-key", "basic-auth", "signed-url")},
					},
					"hop":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), Validators: []validator.Int64{int64validator.Between(1, 10)}},
					"resources": withSize(strList("Audit-log identifiers whose reads mean this decoy was found: a secret ARN, ssm:<name>, aws-access-key:<id>, a GCP secret name, kv:<vault>/<secret>.")),
					"locations": strList("Where the decoy is planted, for the incident brief."),
				}},
			},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"trap_url":   schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"ingest_key": schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keep, Description: "Key for the audit forwarder (null without access_detection)."},
			"ingest_url": schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "Base URL for the forwarder: append /aws, /gcp, /azure or /host."},
		},
	}
}

func withSize(a schema.ListAttribute) schema.ListAttribute {
	a.Validators = []validator.List{listvalidator.SizeAtMost(20)}
	return a
}

func (r *deploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*Client); ok {
		r.client = c
	}
}

func (r *deploymentResource) requireKey(diags *diag.Diagnostics) bool {
	if r.client.APIKey == "" {
		diags.AddError("Missing Lilytrap API key", "Set api_key in the provider block or LILYTRAP_API_KEY. Find it in the dashboard under Settings.")
		return false
	}
	return true
}

func (r *deploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !r.requireKey(&resp.Diagnostics) {
		return
	}
	trapURL, err := r.client.TrapURL(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Couldn't reach Lilytrap", err.Error())
		return
	}
	manifest, ingestKey, diags := buildManifest(ctx, plan, trapURL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Register(ctx, manifest); err != nil {
		resp.Diagnostics.AddError("Lilytrap rejected the deployment", err.Error())
		return
	}
	plan.ID = types.StringValue(manifest.BuildID)
	plan.CreatedAt = types.StringValue(manifest.CreatedAt)
	plan.TrapURL = types.StringValue(trapURL)
	plan.IngestURL = types.StringValue(trimSlash(r.client.APIURL) + "/ingest/v1")
	plan.IngestKey = types.StringNull()
	if ingestKey != "" {
		plan.IngestKey = types.StringValue(ingestKey)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// buildManifest turns the plan into the manifest the API expects. The ingest key is generated
// here and only its hash is sent.
func buildManifest(ctx context.Context, plan deploymentModel, trapURL string) (Manifest, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	list := func(l types.List) []string {
		var out []string
		if !l.IsNull() && !l.IsUnknown() {
			diags.Append(l.ElementsAs(ctx, &out, false)...)
		}
		return out
	}
	m := Manifest{
		Version:   1,
		BuildID:   "bld_tfp_" + randomHex(10),
		CreatedAt: nowISO(),
		Target:    plan.Target.ValueString(),
		Endpoint:  trapURL,
		Source: &Source{
			Kind:     "terraform",
			Name:     plan.Name.ValueString(),
			Location: plan.Location.ValueString(),
			Trusted:  list(plan.TrustedIdentities),
		},
	}
	ingestKey := ""
	if plan.AccessDetection.ValueBool() {
		ingestKey = "lti_" + randomToken(30)
		m.IngestKeyHash = sha256Hex(ingestKey)
	}
	for _, d := range plan.Decoys {
		secret := d.Secret.ValueString()
		locations := list(d.Locations)
		if locations == nil {
			locations = []string{}
		}
		m.Tokens = append(m.Tokens, Token{
			ID:         "tok_" + sha256Hex(secret + ":id")[:14],
			Kit:        d.Kit.ValueString(),
			Kind:       d.Kind.ValueString(),
			SecretHash: sha256Hex(secret),
			Path:       d.Path.ValueString(),
			Method:     d.Method.ValueString(),
			Tells:      []string{},
			Locations:  locations,
			Hop:        d.Hop.ValueInt64(),
			Resources:  list(d.Resources),
		})
	}
	return m, ingestKey, diags
}

// ModifyPlan refuses, at plan time, decoys whose `paths` an ignore rule excludes. Unknown values
// (paths computed from other resources) are checked once they're known, on the next plan.
func (r *deploymentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || r.client == nil || r.client.APIKey == "" {
		return // destroy, or not configured yet
	}
	var plan deploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Paths.IsNull() || plan.Paths.IsUnknown() || plan.Ignore.IsUnknown() {
		return
	}
	var paths, extra []string
	resp.Diagnostics.Append(plan.Paths.ElementsAs(ctx, &paths, false)...)
	if !plan.Ignore.IsNull() {
		resp.Diagnostics.Append(plan.Ignore.ElementsAs(ctx, &extra, false)...)
	}
	if resp.Diagnostics.HasError() || len(paths) == 0 {
		return
	}
	ignored, err := r.client.CheckPolicy(ctx, paths, extra)
	if errors.Is(err, ErrNotFound) {
		if len(extra) > 0 {
			resp.Diagnostics.AddAttributeError(path.Root("ignore"), "Lilytrap can't check ignore rules", "This Lilytrap API predates ignore rules, so `ignore` can't be applied.")
		}
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Couldn't read Lilytrap's ignore rules", err.Error())
		return
	}
	for _, p := range ignored {
		resp.Diagnostics.AddAttributeError(path.Root("paths"), "Decoy location is ignored",
			fmt.Sprintf("Lilytrap ignore rules exclude %s. Move the decoy, or drop it from this deployment (data.lilytrap_policy can filter paths).", p))
	}
}

func (r *deploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !r.requireKey(&resp.Diagnostics) {
		return
	}
	_, err := r.client.GetBuild(ctx, state.ID.ValueString())
	if errors.Is(err, ErrNotFound) {
		// Retired from the dashboard: plan a new deployment.
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Couldn't read the Lilytrap deployment", err.Error())
		return
	}
	// Every refresh is a sign the decoys are still there; without this they'd show as stale.
	if err := r.client.TouchBuild(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddWarning("Couldn't tell Lilytrap this deployment is still there", err.Error())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update only runs for changes that need no replacement, of which there are none.
func (r *deploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state deploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *deploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !r.requireKey(&resp.Diagnostics) {
		return
	}
	if err := r.client.RetireBuild(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Couldn't retire the Lilytrap deployment", err.Error())
	}
}
