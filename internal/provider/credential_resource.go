package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Prefixes of real providers' keys. Decoys never imitate them: a lookalike could trip someone
// else's secret scanning, or be mistaken for a real leak of their product.
var thirdPartyPrefixes = []string{"akia", "asia", "ghp_", "gho_", "ghu_", "ghs_", "github_pat_", "glpat-", "xox", "sk_live", "sk_test", "rk_live", "rk_test", "pk_live", "sk-", "aiza", "ya29.", "npm_", "pypi-", "hf_", "wsk_", "lti_"}

type credentialResource struct{ client *Client }

type credentialModel struct {
	ID         types.String `tfsdk:"id"`
	Prefix     types.String `tfsdk:"prefix"`
	PathPrefix types.String `tfsdk:"path_prefix"`
	Keepers    types.Map    `tfsdk:"keepers"`
	Secret     types.String `tfsdk:"secret"`
	Path       types.String `tfsdk:"path"`
	URL        types.String `tfsdk:"url"`
	TrapURL    types.String `tfsdk:"trap_url"`
}

func newCredentialResource() resource.Resource { return &credentialResource{} }

func (r *credentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_decoy_credential"
}

func (r *credentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A decoy credential and the trap URL it unlocks. Generated locally and kept in Terraform state; " +
			"register it with lilytrap_deployment. Put it wherever an intruder (or a wandering agent) would look.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"prefix": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("adm_"), PlanModifiers: replace,
				Description: "Prefix of the secret, e.g. adm_ or svc_. Prefixes of real providers' keys (AKIA, ghp_, sk_live, ...) are refused.",
				Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,11}[_-]$`), "letters and digits ending in _ or -"), notThirdParty{}},
			},
			"path_prefix": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("/internal/platform-admin"), PlanModifiers: replace,
				Description: "Path on the trap the credential points at; a random segment is appended.",
				Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^(/[A-Za-z0-9._-]+){1,6}$`), "a path like /internal/platform-admin")},
			},
			"keepers": schema.MapAttribute{
				ElementType: types.StringType, Optional: true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
				Description:   "Change any value to rotate the credential.",
			},
			"secret":   schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keep, Description: "The decoy secret."},
			"path":     schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "Path on the trap."},
			"url":      schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "trap_url + path: the admin API the decoy claims to unlock."},
			"trap_url": schema.StringAttribute{Computed: true, PlanModifiers: keep},
		},
	}
}

func (r *credentialResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*Client); ok {
		r.client = c
	}
}

func (r *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan credentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	trapURL, err := r.client.TrapURL(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Couldn't reach Lilytrap", err.Error())
		return
	}
	secret := plan.Prefix.ValueString() + randomToken(30)
	p := fmt.Sprintf("%s/%s/v1", plan.PathPrefix.ValueString(), randomHex(4))
	plan.ID = types.StringValue("cred_" + sha256Hex(secret + ":id")[:14])
	plan.Secret = types.StringValue(secret)
	plan.Path = types.StringValue(p)
	plan.TrapURL = types.StringValue(trapURL)
	plan.URL = types.StringValue(trapURL + p)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read, Update and Delete are local: the credential lives only in Terraform state.
func (r *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state credentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan credentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *credentialResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

type notThirdParty struct{}

func (notThirdParty) Description(context.Context) string {
	return "must not imitate another provider's key format"
}
func (v notThirdParty) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (notThirdParty) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if imitatesThirdParty(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("prefix"), "Prefix imitates another provider's key",
			"Decoys use their own formats so they never trip other providers' secret scanning or get mistaken for a real leak of their product.")
	}
}

func imitatesThirdParty(prefix string) bool {
	p := strings.ToLower(prefix)
	for _, bad := range thirdPartyPrefixes {
		if strings.HasPrefix(p, bad) || strings.HasPrefix(bad, p) && len(p) >= 3 {
			return true
		}
	}
	return false
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }
