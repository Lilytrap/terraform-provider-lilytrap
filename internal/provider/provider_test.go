package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPrefixesNeverImitateRealProviders(t *testing.T) {
	for _, p := range []string{"AKIA", "ghp_", "sk_", "sk_live_", "xoxb-", "glpat-", "wsk_"} {
		if !imitatesThirdParty(p) {
			t.Errorf("%q should be refused", p)
		}
	}
	for _, p := range []string{"adm_", "svc_", "ops-", "platform_"} {
		if imitatesThirdParty(p) {
			t.Errorf("%q should be allowed", p)
		}
	}
}

func TestManifestCarriesOnlyHashes(t *testing.T) {
	ctx := context.Background()
	res, _ := types.ListValueFrom(ctx, types.StringType, []string{"secretsmanager:prod/platform/admin-api"})
	plan := deploymentModel{
		Name:              types.StringValue("aws/prod"),
		Target:            types.StringValue("cloud"),
		Location:          types.StringValue("aws:111122223333/us-east-2"),
		TrustedIdentities: types.ListNull(types.StringType),
		AccessDetection:   types.BoolValue(true),
		Decoys: []decoyModel{{
			Kit: types.StringValue("cloud-secret"), Secret: types.StringValue("adm_super-secret-decoy-value"),
			Path: types.StringValue("/internal/platform-admin/abcd/v1/clusters"), Method: types.StringValue("GET"),
			Kind: types.StringValue("bearer"), Hop: types.Int64Value(1), Resources: res, Locations: types.ListNull(types.StringType),
		}},
	}
	m, ingestKey, diags := buildManifest(ctx, plan, "https://trap.example.net")
	if diags.HasError() {
		t.Fatal(diags)
	}
	raw, _ := json.Marshal(m)
	body := string(raw)
	if strings.Contains(body, "super-secret") || strings.Contains(body, ingestKey) {
		t.Fatal("a plaintext secret would leave Terraform")
	}
	if m.Tokens[0].SecretHash != sha256Hex("adm_super-secret-decoy-value") || m.IngestKeyHash != sha256Hex(ingestKey) {
		t.Fatal("hashes don't match the secrets")
	}
	if !strings.HasPrefix(m.BuildID, "bld_") || m.Source.Kind != "terraform" || m.Tokens[0].Locations == nil {
		t.Fatalf("manifest shape: %s", body)
	}
}
