package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAlertDataSource_UnknownID(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	ds := &alertDataSource{}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned unexpected diagnostics: %v", schemaResp.Diagnostics)
	}

	attrTypes := make(map[string]tftypes.Type)
	configValues := make(map[string]tftypes.Value)

	for name, attr := range schemaResp.Schema.Attributes {
		tfType := attr.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if name == "id" {
			configValues[name] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		} else {
			configValues[name] = tftypes.NewValue(tfType, nil)
		}
	}

	configVal := tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, configValues)
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    configVal,
	}

	readResp := &datasource.ReadResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	ds.Read(ctx, datasource.ReadRequest{Config: config}, readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read() returned unexpected diagnostics: %v", readResp.Diagnostics)
	}

	var data alertDataSourceModel
	if diags := readResp.State.Get(ctx, &data); diags.HasError() {
		t.Fatalf("failed to read state: %v", diags)
	}

	if !data.RecipientsSlackChannels.IsUnknown() {
		t.Errorf("expected RecipientsSlackChannels to be unknown when id is unknown, got %v", data.RecipientsSlackChannels)
	}
}
