package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

func TestToAlertRequest_SlackChannelsAndIgnoreValuesRange(t *testing.T) {
	ctx := t.Context()

	chVal, d := resource_alert.NewRecipientsSlackChannelsValue(
		resource_alert.RecipientsSlackChannelsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"customer_id": types.StringNull(),
			"id":          types.StringValue("C12345"),
			"name":        types.StringNull(),
			"shared":      types.BoolValue(true),
			"type":        types.StringNull(),
			"workspace":   types.StringNull(),
		},
	)
	if d.HasError() {
		t.Fatalf("NewRecipientsSlackChannelsValue: %v", d)
	}

	channelsList, d := types.ListValueFrom(
		ctx,
		resource_alert.RecipientsSlackChannelsValue{}.Type(ctx),
		[]resource_alert.RecipientsSlackChannelsValue{chVal},
	)
	if d.HasError() {
		t.Fatalf("channelsList: %v", d)
	}

	ivrVal, d := resource_alert.NewIgnoreValuesRangeValue(
		resource_alert.IgnoreValuesRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"lower_bound": types.Float64Value(5.0),
			"upper_bound": types.Float64Value(25.0),
		},
	)
	if d.HasError() {
		t.Fatalf("ivrVal: %v", d)
	}

	metricVal, d := resource_alert.NewMetricValue(
		resource_alert.MetricValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"type":  types.StringValue("basic"),
			"value": types.StringValue("cost"),
		},
	)
	if d.HasError() {
		t.Fatalf("metricVal: %v", d)
	}

	emptyScopes, d := types.ListValueFrom(ctx, resource_alert.ScopesValue{}.Type(ctx), []resource_alert.ScopesValue{})
	if d.HasError() {
		t.Fatalf("emptyScopes: %v", d)
	}

	configVal, d := resource_alert.NewConfigValue(
		resource_alert.ConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"condition":           types.StringValue("percentage-change"),
			"currency":            types.StringNull(),
			"data_source":         types.StringNull(),
			"evaluate_for_each":   types.StringNull(),
			"ignore_values_range": ivrVal,
			"metric":              metricVal,
			"operator":            types.StringValue("gt"),
			"scopes":              emptyScopes,
			"time_interval":       types.StringValue("month"),
			"value":               types.Float64Value(10.0),
		},
	)
	if d.HasError() {
		t.Fatalf("configVal: %v", d)
	}

	recipientsList, d := types.ListValueFrom(ctx, types.StringType, []string{"user@example.com"})
	if d.HasError() {
		t.Fatalf("recipientsList: %v", d)
	}

	plan := &alertResourceModel{
		Name:                    types.StringValue("Test Alert"),
		Recipients:              recipientsList,
		RecipientsSlackChannels: channelsList,
		Config:                  configVal,
	}

	req, diags := plan.toAlertRequest(ctx)
	if diags.HasError() {
		t.Fatalf("toAlertRequest: %v", diags)
	}

	if req.RecipientsSlackChannels == nil || len(*req.RecipientsSlackChannels) != 1 {
		t.Fatalf("expected 1 Slack channel, got %v", req.RecipientsSlackChannels)
	}
	ch := (*req.RecipientsSlackChannels)[0]
	if ch.Id != "C12345" {
		t.Errorf("channel id = %q, want C12345", ch.Id)
	}
	if ch.Shared == nil || !*ch.Shared {
		t.Errorf("channel shared = %v, want true", ch.Shared)
	}

	if req.Config.IgnoreValuesRange.IsNull() {
		t.Fatal("expected IgnoreValuesRange to be set")
	}
	ivr, err := req.Config.IgnoreValuesRange.Get()
	if err != nil {
		t.Fatalf("IgnoreValuesRange.Get(): %v", err)
	}
	if ivr.LowerBound != 5.0 || ivr.UpperBound != 25.0 {
		t.Errorf("IgnoreValuesRange = %+v, want lower=5.0, upper=25.0", ivr)
	}
	if req.Config.Operator != "gt" {
		t.Errorf("Operator = %q, want gt", req.Config.Operator)
	}
}

func TestToAlertUpdateRequest_ClearableAttributes(t *testing.T) {
	ctx := t.Context()

	metricVal, d := resource_alert.NewMetricValue(
		resource_alert.MetricValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"type":  types.StringValue("basic"),
			"value": types.StringValue("cost"),
		},
	)
	if d.HasError() {
		t.Fatalf("metricVal: %v", d)
	}

	emptyScopes, d := types.ListValueFrom(ctx, resource_alert.ScopesValue{}.Type(ctx), []resource_alert.ScopesValue{})
	if d.HasError() {
		t.Fatalf("emptyScopes: %v", d)
	}

	// Case 1: ignore_values_range is Null, recipients_slack_channels is Null
	configValNull, d := resource_alert.NewConfigValue(
		resource_alert.ConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"condition":           types.StringValue("percentage-change"),
			"currency":            types.StringNull(),
			"data_source":         types.StringNull(),
			"evaluate_for_each":   types.StringNull(),
			"ignore_values_range": resource_alert.NewIgnoreValuesRangeValueNull(),
			"metric":              metricVal,
			"operator":            types.StringValue("gt"),
			"scopes":              emptyScopes,
			"time_interval":       types.StringValue("month"),
			"value":               types.Float64Value(10.0),
		},
	)
	if d.HasError() {
		t.Fatalf("configValNull: %v", d)
	}

	planNull := &alertResourceModel{
		Name:                    types.StringValue("Test Alert"),
		Recipients:              types.ListNull(types.StringType),
		RecipientsSlackChannels: types.ListNull(resource_alert.RecipientsSlackChannelsValue{}.Type(ctx)),
		Config:                  configValNull,
	}

	reqNull, diags := planNull.toAlertUpdateRequest(ctx)
	if diags.HasError() {
		t.Fatalf("toAlertUpdateRequest: %v", diags)
	}

	// Should send empty slice to clear Slack channels
	if reqNull.RecipientsSlackChannels == nil {
		t.Fatal("expected RecipientsSlackChannels to be non-nil empty slice")
	}
	if len(*reqNull.RecipientsSlackChannels) != 0 {
		t.Errorf("expected 0 Slack channels, got %d", len(*reqNull.RecipientsSlackChannels))
	}

	// Should have IgnoreValuesRange.IsNull() == true
	if reqNull.Config == nil {
		t.Fatal("expected Config to be non-nil")
	}
	if !reqNull.Config.IgnoreValuesRange.IsNull() {
		t.Error("expected IgnoreValuesRange to be null in update request")
	}

	// Case 2: ignore_values_range is Populated, recipients_slack_channels is Populated
	ivrVal, d := resource_alert.NewIgnoreValuesRangeValue(
		resource_alert.IgnoreValuesRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"lower_bound": types.Float64Value(12.0),
			"upper_bound": types.Float64Value(40.0),
		},
	)
	if d.HasError() {
		t.Fatalf("ivrVal: %v", d)
	}

	chVal, d := resource_alert.NewRecipientsSlackChannelsValue(
		resource_alert.RecipientsSlackChannelsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"customer_id": types.StringNull(),
			"id":          types.StringValue("C99999"),
			"name":        types.StringNull(),
			"shared":      types.BoolValue(false),
			"type":        types.StringNull(),
			"workspace":   types.StringValue("my-ws"),
		},
	)
	if d.HasError() {
		t.Fatalf("chVal: %v", d)
	}

	channelsPopulated, d := types.ListValueFrom(
		ctx,
		resource_alert.RecipientsSlackChannelsValue{}.Type(ctx),
		[]resource_alert.RecipientsSlackChannelsValue{chVal},
	)
	if d.HasError() {
		t.Fatalf("channelsPopulated: %v", d)
	}

	configValPopulated, d := resource_alert.NewConfigValue(
		resource_alert.ConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"condition":           types.StringValue("percentage-change"),
			"currency":            types.StringNull(),
			"data_source":         types.StringNull(),
			"evaluate_for_each":   types.StringNull(),
			"ignore_values_range": ivrVal,
			"metric":              metricVal,
			"operator":            types.StringValue("gt"),
			"scopes":              emptyScopes,
			"time_interval":       types.StringValue("month"),
			"value":               types.Float64Value(10.0),
		},
	)
	if d.HasError() {
		t.Fatalf("configValPopulated: %v", d)
	}

	planPopulated := &alertResourceModel{
		Name:                    types.StringValue("Test Alert"),
		Recipients:              types.ListNull(types.StringType),
		RecipientsSlackChannels: channelsPopulated,
		Config:                  configValPopulated,
	}

	reqPopulated, diags := planPopulated.toAlertUpdateRequest(ctx)
	if diags.HasError() {
		t.Fatalf("toAlertUpdateRequest: %v", diags)
	}

	if reqPopulated.RecipientsSlackChannels == nil || len(*reqPopulated.RecipientsSlackChannels) != 1 {
		t.Fatalf("expected 1 Slack channel, got %v", reqPopulated.RecipientsSlackChannels)
	}
	chPop := (*reqPopulated.RecipientsSlackChannels)[0]
	if chPop.Id != "C99999" || chPop.Workspace == nil || *chPop.Workspace != "my-ws" {
		t.Errorf("channel = %+v, want id=C99999, workspace=my-ws", chPop)
	}

	if reqPopulated.Config.IgnoreValuesRange.IsNull() {
		t.Fatal("expected IgnoreValuesRange to not be null")
	}
	ivrPop, err := reqPopulated.Config.IgnoreValuesRange.Get()
	if err != nil {
		t.Fatalf("IgnoreValuesRange.Get(): %v", err)
	}
	if ivrPop.LowerBound != 12.0 || ivrPop.UpperBound != 40.0 {
		t.Errorf("IgnoreValuesRange = %+v, want lower=12.0, upper=40.0", ivrPop)
	}
}

func TestMapAlertToModel_SlackChannelsAndLastAlerted(t *testing.T) {
	ctx := t.Context()

	var lastAlertedVal int64 = 1718000000
	var lastAlertedNullable nullable.Nullable[int64]
	lastAlertedNullable.Set(lastAlertedVal)

	var ivrNullable nullable.Nullable[models.AlertIgnoreValuesRange]
	ivrNullable.Set(models.AlertIgnoreValuesRange{
		LowerBound: 3.5,
		UpperBound: 15.5,
	})

	idVal := "alert-123"
	custID := "cust-1"
	name := "Alert Test"
	ws := "workspace-x"
	shared := false
	chType := models.AlertSlackChannelTypePublic

	apiAlert := &models.Alert{
		Id:          &idVal,
		Name:        "Test Alert",
		LastAlerted: lastAlertedNullable,
		RecipientsSlackChannels: &[]models.AlertSlackChannel{
			{
				CustomerId: &custID,
				Id:         "C55555",
				Name:       &name,
				Shared:     &shared,
				Type:       &chType,
				Workspace:  &ws,
			},
		},
		Config: &models.AlertConfig{
			Condition:         new(models.Condition),
			IgnoreValuesRange: ivrNullable,
			Metric: models.MetricConfig{
				Type:  "basic",
				Value: "cost",
			},
			Operator:     models.MetricFilterTextGt,
			TimeInterval: models.AlertConfigTimeIntervalMonth,
			Value:        250.0,
		},
	}
	*apiAlert.Config.Condition = models.ConditionPercentageChange

	state := &alertResourceModel{}
	diags := mapAlertToModel(ctx, apiAlert, state)
	if diags.HasError() {
		t.Fatalf("mapAlertToModel: %v", diags)
	}

	if state.LastAlerted.IsNull() || state.LastAlerted.ValueInt64() != lastAlertedVal {
		t.Errorf("LastAlerted = %v, want %d", state.LastAlerted, lastAlertedVal)
	}

	if state.RecipientsSlackChannels.IsNull() || len(state.RecipientsSlackChannels.Elements()) != 1 {
		t.Fatalf("expected 1 Slack channel in state, got %v", state.RecipientsSlackChannels)
	}

	if state.Config.IsNull() {
		t.Fatal("expected Config in state to not be null")
	}
	if state.Config.IgnoreValuesRange.IsNull() {
		t.Fatal("expected IgnoreValuesRange in state to not be null")
	}
	if state.Config.IgnoreValuesRange.LowerBound.ValueFloat64() != 3.5 {
		t.Errorf("LowerBound = %v, want 3.5", state.Config.IgnoreValuesRange.LowerBound)
	}
	if state.Config.IgnoreValuesRange.UpperBound.ValueFloat64() != 15.5 {
		t.Errorf("UpperBound = %v, want 15.5", state.Config.IgnoreValuesRange.UpperBound)
	}
	if state.Config.Operator.ValueString() != "gt" {
		t.Errorf("Operator = %q, want gt", state.Config.Operator.ValueString())
	}
}

func TestUseNullForIgnoreValuesRangeModifier(t *testing.T) {
	ctx := t.Context()
	m := useNullForIgnoreValuesRange()

	// Case 1: Config is null -> plan value should be set to null
	req := planmodifier.ObjectRequest{
		ConfigValue: types.ObjectNull(map[string]attr.Type{
			"lower_bound": types.Float64Type,
			"upper_bound": types.Float64Type,
		}),
		PlanValue: types.ObjectUnknown(map[string]attr.Type{
			"lower_bound": types.Float64Type,
			"upper_bound": types.Float64Type,
		}),
	}
	resp := &planmodifier.ObjectResponse{
		PlanValue: req.PlanValue,
	}
	m.PlanModifyObject(ctx, req, resp)
	if !resp.PlanValue.IsNull() {
		t.Errorf("expected plan value to be null when config is null, got %v", resp.PlanValue)
	}

	// Case 2: Config is known/not null -> plan value should not be modified to null
	objVal, d := types.ObjectValue(
		map[string]attr.Type{
			"lower_bound": types.Float64Type,
			"upper_bound": types.Float64Type,
		},
		map[string]attr.Value{
			"lower_bound": types.Float64Value(1.0),
			"upper_bound": types.Float64Value(2.0),
		},
	)
	if d.HasError() {
		t.Fatalf("ObjectValue: %v", d)
	}
	req2 := planmodifier.ObjectRequest{
		ConfigValue: objVal,
		PlanValue:   objVal,
	}
	resp2 := &planmodifier.ObjectResponse{
		PlanValue: req2.PlanValue,
	}
	m.PlanModifyObject(ctx, req2, resp2)
	if resp2.PlanValue.IsNull() {
		t.Errorf("expected plan value to not be null when config is populated")
	}
}
