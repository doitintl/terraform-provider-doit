package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_alerts"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestAlertsDataSource_MapAlertsSlackChannels_SlackNormalization(t *testing.T) {
	ctx := t.Context()

	emptyWs := ""
	apiChannels := &[]models.AlertSlackChannel{
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
	}

	var diags diag.Diagnostics
	channelsList := mapAlertsSlackChannels(ctx, apiChannels, &diags)
	if diags.HasError() {
		t.Fatalf("mapAlertsSlackChannels failed: %v", diags)
	}

	var channels []datasource_alerts.RecipientsSlackChannelsValue
	diags = channelsList.ElementsAs(ctx, &channels, false)
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
