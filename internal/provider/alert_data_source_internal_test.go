package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_alert"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
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

func TestAlertDataSource_MapAlertResponseToModel_SlackNormalization(t *testing.T) {
	ctx := t.Context()
	ds := &alertDataSource{}

	emptyWs := ""
	apiAlert := &models.Alert{
		Id:   new("alert-123"),
		Name: "Test Alert",
		RecipientsSlackChannels: &[]models.AlertSlackChannel{
			{
				Id:        "C11111",
				Workspace: new("my-ws"),
				Shared:    nil, // workspace channel with omitted shared
			},
			{
				Id:        "C22222",
				Workspace: &emptyWs, // shared channel with empty workspace string
				Shared:    new(true),
			},
		},
	}

	var state datasource_alert.AlertModel
	diags := ds.mapAlertToModel(ctx, apiAlert, &state)
	if diags.HasError() {
		t.Fatalf("mapAlertToModel failed: %v", diags)
	}

	var channels []datasource_alert.RecipientsSlackChannelsValue
	diags = state.RecipientsSlackChannels.ElementsAs(ctx, &channels, false)
	if diags.HasError() {
		t.Fatalf("ElementsAs failed: %v", diags)
	}

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	// Channel 0: workspace channel should normalize nil Shared to false
	if channels[0].Shared.IsNull() {
		t.Errorf("channel 0: expected shared to be false, got null")
	} else if channels[0].Shared.ValueBool() != false {
		t.Errorf("channel 0: expected shared to be false, got %v", channels[0].Shared.ValueBool())
	}

	// Channel 1: shared channel should normalize empty string Workspace to null
	if !channels[1].Workspace.IsNull() {
		t.Errorf("channel 1: expected workspace to be null, got %v", channels[1].Workspace)
	}
}
