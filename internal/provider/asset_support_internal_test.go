package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_assets"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_asset"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAssetSupportAttrs(t *testing.T) {
	t.Parallel()

	partnerLed := models.AssetSupportModelPartnerLed
	resold := models.AssetSupportModelResold
	businessPlus := models.AssetSupportTierBusinessPlus
	noSupport := models.AssetSupportTierNoSupport

	tests := []struct {
		name string
		in   *models.AssetSupport
		want map[string]attr.Value
	}{
		{"nil", nil, map[string]attr.Value{"model": types.StringNull(), "tier": types.StringNull()}},
		{"empty", &models.AssetSupport{}, map[string]attr.Value{"model": types.StringNull(), "tier": types.StringNull()}},
		{"model only", &models.AssetSupport{Model: &resold}, map[string]attr.Value{"model": types.StringValue("resold"), "tier": types.StringNull()}},
		{"tier only", &models.AssetSupport{Tier: &businessPlus}, map[string]attr.Value{"model": types.StringNull(), "tier": types.StringValue("business_plus")}},
		{"aws", &models.AssetSupport{Model: &resold, Tier: &businessPlus}, map[string]attr.Value{"model": types.StringValue("resold"), "tier": types.StringValue("business_plus")}},
		{"gcp", &models.AssetSupport{Model: &partnerLed, Tier: &noSupport}, map[string]attr.Value{"model": types.StringValue("partner_led"), "tier": types.StringValue("no_support")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := assetSupportAttrs(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d attrs, want %d", len(got), len(tt.want))
			}
			for k, w := range tt.want {
				if !got[k].Equal(w) {
					t.Errorf("%s = %s, want %s", k, got[k], w)
				}
			}
		})
	}
}

func supportValue(t *testing.T, model, tier string) resource_asset.SupportValue {
	t.Helper()
	v, d := resource_asset.NewSupportValue(resource_asset.SupportValue{}.AttributeTypes(t.Context()), map[string]attr.Value{
		"model": types.StringValue(model),
		"tier":  types.StringValue(tier),
	})
	if d.HasError() {
		t.Fatalf("building expected support: %v", d)
	}
	return v
}

func TestMapAssetToModel_Support(t *testing.T) {
	t.Parallel()

	model := models.AssetSupportModelResold
	tier := models.AssetSupportTierBasic
	support := &models.AssetSupport{Model: &model, Tier: &tier}

	tests := []struct {
		name  string
		asset *models.AssetItemDetailed
		want  resource_asset.SupportValue
	}{
		{
			// mapAssetToModel returns early without properties; support must still be mapped.
			name:  "support without properties",
			asset: &models.AssetItemDetailed{Support: support},
			want:  supportValue(t, "resold", "basic"),
		},
		{
			name:  "support with properties",
			asset: &models.AssetItemDetailed{Support: support, Properties: &models.AssetProperties{}},
			want:  supportValue(t, "resold", "basic"),
		},
		{
			name:  "no support",
			asset: &models.AssetItemDetailed{Properties: &models.AssetProperties{}},
			want:  resource_asset.NewSupportValueNull(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var state assetResourceModel
			if d := mapAssetToModel(t.Context(), tt.asset, &state); d.HasError() {
				t.Fatalf("mapAssetToModel: %v", d)
			}
			if !state.Support.Equal(tt.want) {
				t.Errorf("Support = %s, want %s", state.Support, tt.want)
			}
		})
	}
}

func TestOverlayAssetComputedFields_Support(t *testing.T) {
	t.Parallel()

	model := models.AssetSupportModelPartnerLed
	tier := models.AssetSupportTierNoSupport
	api := &models.AssetItemDetailed{Support: &models.AssetSupport{Model: &model, Tier: &tier}}

	for name, planned := range map[string]resource_asset.SupportValue{
		"unknown": resource_asset.NewSupportValueUnknown(),
		"stale":   supportValue(t, "resold", "premium"),
		"null":    resource_asset.NewSupportValueNull(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			plan := assetResourceModel{Support: planned}
			if d := overlayAssetComputedFields(t.Context(), api, &plan); d.HasError() {
				t.Fatalf("overlay: %v", d)
			}
			if want := supportValue(t, "partner_led", "no_support"); !plan.Support.Equal(want) {
				t.Errorf("Support = %s, want %s", plan.Support, want)
			}
		})
	}
}

// TestAssetsDataSource_ReadMapsSupport guards the list mapping: the generated
// NewAssetsValue requires a support entry, so a missing key fails every read.
func TestAssetsDataSource_ReadMapsSupport(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rowCount":2,"assets":[
			{"id":"amazon-web-services-1","type":"amazon-web-services","support":{"model":"resold","tier":"basic"}},
			{"id":"g-suite-1","type":"g-suite"}]}`))
	}))
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	ds := &assetsDataSource{client: client}
	ctx := t.Context()
	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, schemaResp)

	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, a := range schemaResp.Schema.Attributes {
		tfType := a.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		values[name] = tftypes.NewValue(tfType, nil)
	}
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}
	readResp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: config}, readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", readResp.Diagnostics)
	}

	var data assetsDataSourceModel
	if d := readResp.State.Get(ctx, &data); d.HasError() {
		t.Fatalf("state: %v", d)
	}
	var items []datasource_assets.AssetsValue
	if d := data.Assets.ElementsAs(ctx, &items, false); d.HasError() || len(items) != 2 {
		t.Fatalf("assets: %v (len %d)", d, len(items))
	}

	wantSupport, d := datasource_assets.NewSupportValue(datasource_assets.SupportValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"model": types.StringValue("resold"),
		"tier":  types.StringValue("basic"),
	})
	if d.HasError() {
		t.Fatalf("expected support: %v", d)
	}
	if !items[0].Support.Equal(wantSupport) {
		t.Errorf("assets[0].support = %s, want %s", items[0].Support, wantSupport)
	}
	if !items[1].Support.IsNull() {
		t.Errorf("assets[1].support = %s, want null", items[1].Support)
	}
}
