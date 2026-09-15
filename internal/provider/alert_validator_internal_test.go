package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func buildAlertValidatorConfig(
	ctx context.Context,
	t *testing.T,
	recipients attr.Value,
	slackChannels attr.Value,
	configVal attr.Value,
) tfsdk.Config {
	t.Helper()
	schema := resource_alert.AlertResourceSchema(ctx)

	schemaType := schema.Type().TerraformType(ctx)
	objType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}

	attrValues := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrValues[name] = tftypes.NewValue(attrType, nil)
	}

	if recipients != nil {
		tfVal, err := recipients.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("recipients ToTerraformValue: %v", err)
		}
		attrValues["recipients"] = tfVal
	}
	if slackChannels != nil {
		tfVal, err := slackChannels.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("slackChannels ToTerraformValue: %v", err)
		}
		attrValues["recipients_slack_channels"] = tfVal
	}
	if configVal != nil {
		tfVal, err := configVal.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("configVal ToTerraformValue: %v", err)
		}
		attrValues["config"] = tfVal
	}

	return tfsdk.Config{
		Schema: schema,
		Raw:    tftypes.NewValue(schemaType, attrValues),
	}
}

func assertAlertValidatorDiagnostics(t *testing.T, diagnostics diag.Diagnostics, wantError bool, wantSummary string, wantPath path.Path) {
	t.Helper()
	wantCount := 0
	if wantError {
		wantCount = 1
	}
	if len(diagnostics) != wantCount {
		t.Fatalf("diagnostic count = %d, want %d: %v", len(diagnostics), wantCount, diagnostics)
	}
	if !wantError {
		return
	}
	diagnostic := diagnostics[0]
	if diagnostic.Severity() != diag.SeverityError {
		t.Errorf("severity = %s, want error", diagnostic.Severity())
	}
	if diagnostic.Summary() != wantSummary {
		t.Errorf("summary = %q, want %q", diagnostic.Summary(), wantSummary)
	}
	withPath, ok := diagnostic.(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("diagnostic %T has no path", diagnostic)
	}
	if got := withPath.Path(); !got.Equal(wantPath) {
		t.Errorf("path = %s, want %s", got, wantPath)
	}
}

func TestAlertResource_ModifyPlan_Destinations(t *testing.T) {
	ctx := t.Context()

	recipientsPopulated, diags := types.ListValueFrom(ctx, types.StringType, []string{"user@example.com"})
	if diags.HasError() {
		t.Fatalf("recipientsPopulated: %v", diags)
	}
	recipientsEmpty, diags := types.ListValueFrom(ctx, types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("recipientsEmpty: %v", diags)
	}
	recipientsNull := types.ListNull(types.StringType)
	recipientsUnknown := types.ListUnknown(types.StringType)

	slackChan, d := resource_alert.NewRecipientsSlackChannelsValue(
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

	slackPopulated, diags := types.ListValueFrom(
		ctx,
		resource_alert.RecipientsSlackChannelsValue{}.Type(ctx),
		[]resource_alert.RecipientsSlackChannelsValue{slackChan},
	)
	if diags.HasError() {
		t.Fatalf("slackPopulated: %v", diags)
	}
	slackEmpty, diags := types.ListValueFrom(
		ctx,
		resource_alert.RecipientsSlackChannelsValue{}.Type(ctx),
		[]resource_alert.RecipientsSlackChannelsValue{},
	)
	if diags.HasError() {
		t.Fatalf("slackEmpty: %v", diags)
	}
	slackNull := types.ListNull(resource_alert.RecipientsSlackChannelsValue{}.Type(ctx))
	slackUnknown := types.ListUnknown(resource_alert.RecipientsSlackChannelsValue{}.Type(ctx))
	slackUnknownElem := types.ListValueMust(
		resource_alert.RecipientsSlackChannelsValue{}.Type(ctx),
		[]attr.Value{resource_alert.NewRecipientsSlackChannelsValueUnknown()},
	)

	tests := []struct {
		name             string
		isCreate         bool
		stateRecipients  attr.Value
		stateSlack       attr.Value
		configRecipients attr.Value
		configSlack      attr.Value
		planRecipients   attr.Value
		planSlack        attr.Value
		wantError        bool
		errorSummary     string
		wantPath         path.Path
	}{
		// CREATE cases
		{
			name:             "create: both omitted (null) is valid (defaults to owner)",
			isCreate:         true,
			configRecipients: recipientsNull,
			configSlack:      slackNull,
			planRecipients:   recipientsNull,
			planSlack:        slackNull,
			wantError:        false,
		},
		{
			name:             "create: recipients omitted (null), slack empty is valid (defaults to owner)",
			isCreate:         true,
			configRecipients: recipientsNull,
			configSlack:      slackEmpty,
			planRecipients:   recipientsNull,
			planSlack:        slackEmpty,
			wantError:        false,
		},
		{
			name:             "create: both unknown is deferred",
			isCreate:         true,
			configRecipients: recipientsNull,
			configSlack:      slackNull,
			planRecipients:   recipientsUnknown,
			planSlack:        slackUnknown,
			wantError:        false,
		},
		{
			name:             "create: recipients populated, slack null is valid",
			isCreate:         true,
			configRecipients: recipientsPopulated,
			configSlack:      slackNull,
			planRecipients:   recipientsPopulated,
			planSlack:        slackNull,
			wantError:        false,
		},
		{
			name:             "create: recipients populated, slack empty is valid",
			isCreate:         true,
			configRecipients: recipientsPopulated,
			configSlack:      slackEmpty,
			planRecipients:   recipientsPopulated,
			planSlack:        slackEmpty,
			wantError:        false,
		},
		{
			name:             "create: recipients populated, slack unknown is valid",
			isCreate:         true,
			configRecipients: recipientsPopulated,
			configSlack:      slackNull,
			planRecipients:   recipientsPopulated,
			planSlack:        slackUnknown,
			wantError:        false,
		},
		{
			name:             "create: recipients empty, slack populated is valid",
			isCreate:         true,
			configRecipients: recipientsEmpty,
			configSlack:      slackPopulated,
			planRecipients:   recipientsEmpty,
			planSlack:        slackPopulated,
			wantError:        false,
		},
		{
			name:             "create: recipients empty, slack null is invalid",
			isCreate:         true,
			configRecipients: recipientsEmpty,
			configSlack:      slackNull,
			planRecipients:   recipientsEmpty,
			planSlack:        slackNull,
			wantError:        true,
			errorSummary:     "At Least One Recipient Required",
			wantPath:         path.Root("recipients"),
		},
		{
			name:             "create: recipients empty, slack empty is invalid",
			isCreate:         true,
			configRecipients: recipientsEmpty,
			configSlack:      slackEmpty,
			planRecipients:   recipientsEmpty,
			planSlack:        slackEmpty,
			wantError:        true,
			errorSummary:     "At Least One Recipient Required",
			wantPath:         path.Root("recipients"),
		},
		{
			name:             "create: recipients empty, slack with only null element is invalid",
			isCreate:         true,
			configRecipients: recipientsEmpty,
			configSlack:      types.ListValueMust(resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []attr.Value{resource_alert.NewRecipientsSlackChannelsValueNull()}),
			planRecipients:   recipientsEmpty,
			planSlack:        types.ListValueMust(resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []attr.Value{resource_alert.NewRecipientsSlackChannelsValueNull()}),
			wantError:        true,
			errorSummary:     "At Least One Recipient Required",
			wantPath:         path.Root("recipients"),
		},
		{
			name:             "create: recipients unknown is deferred",
			isCreate:         true,
			configRecipients: recipientsEmpty,
			configSlack:      slackEmpty,
			planRecipients:   recipientsUnknown,
			planSlack:        slackEmpty,
			wantError:        false,
		},
		{
			name:             "create: slack unknown is deferred",
			isCreate:         true,
			configRecipients: recipientsEmpty,
			configSlack:      slackUnknown,
			planRecipients:   recipientsEmpty,
			planSlack:        slackUnknown,
			wantError:        false,
		},
		{
			name:             "create: slack element unknown is deferred",
			isCreate:         true,
			configRecipients: recipientsEmpty,
			configSlack:      slackUnknownElem,
			planRecipients:   recipientsEmpty,
			planSlack:        slackUnknownElem,
			wantError:        false,
		},
		// UPDATE cases
		{
			name:             "update: Slack-only alert, removing both destinations is invalid",
			isCreate:         false,
			stateRecipients:  recipientsEmpty,
			stateSlack:       slackPopulated,
			configRecipients: recipientsNull,
			configSlack:      slackNull,
			planRecipients:   recipientsEmpty,
			planSlack:        slackNull,
			wantError:        true,
			errorSummary:     "At Least One Destination Required",
			wantPath:         path.Root("recipients_slack_channels"),
		},
		{
			name:             "update: Slack-only alert, setting recipients populated and removing slack is valid",
			isCreate:         false,
			stateRecipients:  recipientsEmpty,
			stateSlack:       slackPopulated,
			configRecipients: recipientsPopulated,
			configSlack:      slackNull,
			planRecipients:   recipientsPopulated,
			planSlack:        slackNull,
			wantError:        false,
		},
		{
			name:             "update: alert with emails, removing slack is valid (emails retained from state)",
			isCreate:         false,
			stateRecipients:  recipientsPopulated,
			stateSlack:       slackPopulated,
			configRecipients: recipientsNull,
			configSlack:      slackNull,
			planRecipients:   recipientsPopulated,
			planSlack:        slackNull,
			wantError:        false,
		},
		{
			name:             "update: alert with emails, setting recipients to empty and removing slack is invalid",
			isCreate:         false,
			stateRecipients:  recipientsPopulated,
			stateSlack:       slackPopulated,
			configRecipients: recipientsEmpty,
			configSlack:      slackNull,
			planRecipients:   recipientsEmpty,
			planSlack:        slackNull,
			wantError:        true,
			errorSummary:     "At Least One Recipient Required",
			wantPath:         path.Root("recipients"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planCfg := buildAlertValidatorConfig(ctx, t, tt.planRecipients, tt.planSlack, nil)
			configCfg := buildAlertValidatorConfig(ctx, t, tt.configRecipients, tt.configSlack, nil)

			req := resource.ModifyPlanRequest{
				Plan:   tfsdk.Plan(planCfg),
				Config: configCfg,
			}
			if !tt.isCreate {
				stateCfg := buildAlertValidatorConfig(ctx, t, tt.stateRecipients, tt.stateSlack, nil)
				req.State = tfsdk.State(stateCfg)
			}

			resp := &resource.ModifyPlanResponse{}
			r := &alertResource{}
			r.ModifyPlan(ctx, req, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diags: %v)", got, tt.wantError, resp.Diagnostics)
			}
			assertAlertValidatorDiagnostics(t, resp.Diagnostics, tt.wantError, tt.errorSummary, tt.wantPath)
		})
	}
}

func makeTestAlertConfigValue(
	ctx context.Context,
	t *testing.T,
	condition attr.Value,
	ignoreValuesRange attr.Value,
) resource_alert.ConfigValue {
	t.Helper()

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

	configAttrs := map[string]attr.Value{
		"condition":           condition,
		"currency":            types.StringNull(),
		"data_source":         types.StringNull(),
		"evaluate_for_each":   types.StringNull(),
		"ignore_values_range": ignoreValuesRange,
		"metric":              metricVal,
		"operator":            types.StringValue("gt"),
		"scopes":              emptyScopes,
		"time_interval":       types.StringValue("month"),
		"value":               types.Float64Value(100.0),
	}

	configVal, d := resource_alert.NewConfigValue(resource_alert.ConfigValue{}.AttributeTypes(ctx), configAttrs)
	if d.HasError() {
		t.Fatalf("NewConfigValue: %v", d)
	}
	return configVal
}

func TestAlertIgnoreValuesRangeValidator(t *testing.T) {
	ctx := t.Context()

	rangePath := path.Root("config").AtName("ignore_values_range")

	validRange, d := resource_alert.NewIgnoreValuesRangeValue(
		resource_alert.IgnoreValuesRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"lower_bound": types.Float64Value(10.0),
			"upper_bound": types.Float64Value(20.0),
		},
	)
	if d.HasError() {
		t.Fatalf("validRange: %v", d)
	}

	equalRange, d := resource_alert.NewIgnoreValuesRangeValue(
		resource_alert.IgnoreValuesRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"lower_bound": types.Float64Value(15.0),
			"upper_bound": types.Float64Value(15.0),
		},
	)
	if d.HasError() {
		t.Fatalf("equalRange: %v", d)
	}

	invertedRange, d := resource_alert.NewIgnoreValuesRangeValue(
		resource_alert.IgnoreValuesRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"lower_bound": types.Float64Value(30.0),
			"upper_bound": types.Float64Value(10.0),
		},
	)
	if d.HasError() {
		t.Fatalf("invertedRange: %v", d)
	}

	unknownLowerRange, d := resource_alert.NewIgnoreValuesRangeValue(
		resource_alert.IgnoreValuesRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"lower_bound": types.Float64Unknown(),
			"upper_bound": types.Float64Value(10.0),
		},
	)
	if d.HasError() {
		t.Fatalf("unknownLowerRange: %v", d)
	}

	unknownUpperRange, d := resource_alert.NewIgnoreValuesRangeValue(
		resource_alert.IgnoreValuesRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"lower_bound": types.Float64Value(10.0),
			"upper_bound": types.Float64Unknown(),
		},
	)
	if d.HasError() {
		t.Fatalf("unknownUpperRange: %v", d)
	}

	rangeNull := resource_alert.NewIgnoreValuesRangeValueNull()
	rangeUnknown := resource_alert.NewIgnoreValuesRangeValueUnknown()

	tests := []struct {
		name              string
		condition         attr.Value
		ignoreValuesRange attr.Value
		wantError         bool
		errorSummary      string
		wantPath          path.Path
	}{
		{
			name:              "null range with condition percentage-change is valid",
			condition:         types.StringValue("percentage-change"),
			ignoreValuesRange: rangeNull,
			wantError:         false,
		},
		{
			name:              "null range with condition value is valid",
			condition:         types.StringValue("value"),
			ignoreValuesRange: rangeNull,
			wantError:         false,
		},
		{
			name:              "valid range with condition percentage-change is valid",
			condition:         types.StringValue("percentage-change"),
			ignoreValuesRange: validRange,
			wantError:         false,
		},
		{
			name:              "equal bounds with condition percentage-change is valid",
			condition:         types.StringValue("percentage-change"),
			ignoreValuesRange: equalRange,
			wantError:         false,
		},
		{
			name:              "inverted bounds with condition percentage-change is invalid",
			condition:         types.StringValue("percentage-change"),
			ignoreValuesRange: invertedRange,
			wantError:         true,
			errorSummary:      "Invalid Ignore Values Range Bounds",
			wantPath:          rangePath.AtName("lower_bound"),
		},
		{
			name:              "range configured with condition value is invalid",
			condition:         types.StringValue("value"),
			ignoreValuesRange: validRange,
			wantError:         true,
			errorSummary:      "Invalid Ignore Values Range Condition",
			wantPath:          rangePath,
		},
		{
			name:              "range configured with condition forecasted-value is invalid",
			condition:         types.StringValue("forecasted-value"),
			ignoreValuesRange: validRange,
			wantError:         true,
			errorSummary:      "Invalid Ignore Values Range Condition",
			wantPath:          rangePath,
		},
		{
			name:              "range configured with null condition is invalid",
			condition:         types.StringNull(),
			ignoreValuesRange: validRange,
			wantError:         true,
			errorSummary:      "Invalid Ignore Values Range Condition",
			wantPath:          rangePath,
		},
		{
			name:              "range configured with unknown condition is deferred",
			condition:         types.StringUnknown(),
			ignoreValuesRange: validRange,
			wantError:         false,
		},
		{
			name:              "unknown range is deferred",
			condition:         types.StringValue("percentage-change"),
			ignoreValuesRange: rangeUnknown,
			wantError:         false,
		},
		{
			name:              "range with unknown lower bound is deferred",
			condition:         types.StringValue("percentage-change"),
			ignoreValuesRange: unknownLowerRange,
			wantError:         false,
		},
		{
			name:              "range with unknown upper bound is deferred",
			condition:         types.StringValue("percentage-change"),
			ignoreValuesRange: unknownUpperRange,
			wantError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgVal := makeTestAlertConfigValue(ctx, t, tt.condition, tt.ignoreValuesRange)
			cfg := buildAlertValidatorConfig(ctx, t, nil, nil, cfgVal)
			req := resource.ValidateConfigRequest{Config: cfg}
			resp := &resource.ValidateConfigResponse{}
			alertIgnoreValuesRangeValidator{}.ValidateResource(ctx, req, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diags: %v)", got, tt.wantError, resp.Diagnostics)
			}
			assertAlertValidatorDiagnostics(t, resp.Diagnostics, tt.wantError, tt.errorSummary, tt.wantPath)
		})
	}
}

func makeSlackChannel(
	ctx context.Context,
	t *testing.T,
	id attr.Value,
	shared attr.Value,
	workspace attr.Value,
) resource_alert.RecipientsSlackChannelsValue {
	t.Helper()
	v, d := resource_alert.NewRecipientsSlackChannelsValue(
		resource_alert.RecipientsSlackChannelsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"customer_id": types.StringNull(),
			"id":          id,
			"name":        types.StringNull(),
			"shared":      shared,
			"type":        types.StringNull(),
			"workspace":   workspace,
		},
	)
	if d.HasError() {
		t.Fatalf("makeSlackChannel: %v", d)
	}
	return v
}

func TestAlertSlackChannelsValidator(t *testing.T) {
	ctx := t.Context()
	channelsPath := path.Root("recipients_slack_channels")

	elementType := resource_alert.RecipientsSlackChannelsValue{}.Type(ctx)
	list := func(values ...attr.Value) types.List {
		return types.ListValueMust(elementType, values)
	}

	sharedValid := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(true), types.StringNull())
	sharedWithWs := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(true), types.StringValue("my-workspace"))
	wsValid := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(false), types.StringValue("my-workspace"))
	wsWithoutWs := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(false), types.StringNull())
	sharedNullWithWs := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolNull(), types.StringValue("my-workspace"))
	sharedNullWithoutWs := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolNull(), types.StringNull())

	chShared1 := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(true), types.StringNull())
	chShared2 := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(true), types.StringNull())
	chSharedUnknownWs1 := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(true), types.StringUnknown())
	chSharedUnknownWs2 := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(true), types.StringUnknown())
	chWsA1 := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(false), types.StringValue("workspace-a"))
	chWsA2 := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(false), types.StringValue("workspace-a"))
	chWsB := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(false), types.StringValue("workspace-b"))

	unknownShared := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolUnknown(), types.StringValue("workspace-a"))
	unknownWorkspace := makeSlackChannel(ctx, t, types.StringValue("C123"), types.BoolValue(false), types.StringUnknown())
	unknownId1 := makeSlackChannel(ctx, t, types.StringUnknown(), types.BoolValue(true), types.StringNull())
	unknownId2 := makeSlackChannel(ctx, t, types.StringUnknown(), types.BoolValue(true), types.StringNull())

	unknownElement := resource_alert.NewRecipientsSlackChannelsValueUnknown()
	nullElement := resource_alert.NewRecipientsSlackChannelsValueNull()

	tests := []struct {
		name         string
		channels     types.List
		wantError    bool
		errorSummary string
		wantPath     path.Path
	}{
		{
			name:      "unknown list is deferred",
			channels:  types.ListUnknown(elementType),
			wantError: false,
		},
		{
			name:      "unknown element is deferred",
			channels:  list(unknownElement),
			wantError: false,
		},
		{
			name:         "null element is rejected",
			channels:     list(nullElement, sharedValid),
			wantError:    true,
			errorSummary: "Null Slack Channel",
			wantPath:     channelsPath.AtListIndex(0),
		},
		{
			name:      "shared true without workspace is valid",
			channels:  list(sharedValid),
			wantError: false,
		},
		{
			name:         "shared true with workspace is invalid",
			channels:     list(sharedWithWs),
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
			wantPath:     channelsPath.AtListIndex(0).AtName("workspace"),
		},
		{
			name:      "shared false with workspace is valid",
			channels:  list(wsValid),
			wantError: false,
		},
		{
			name:         "shared false without workspace is invalid",
			channels:     list(wsWithoutWs),
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
			wantPath:     channelsPath.AtListIndex(0).AtName("workspace"),
		},
		{
			name:      "shared null (omitted) with workspace is valid",
			channels:  list(sharedNullWithWs),
			wantError: false,
		},
		{
			name:         "shared null (omitted) without workspace is invalid",
			channels:     list(sharedNullWithoutWs),
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
			wantPath:     channelsPath.AtListIndex(0).AtName("workspace"),
		},
		{
			name:      "channel with unknown shared is deferred",
			channels:  list(unknownShared),
			wantError: false,
		},
		{
			name:      "channel with unknown workspace is deferred",
			channels:  list(unknownWorkspace),
			wantError: false,
		},
		{
			name:      "channels with unknown id do not trigger false duplicate",
			channels:  list(unknownId1, unknownId2),
			wantError: false,
		},
		{
			name:         "duplicate shared channel is invalid",
			channels:     list(chShared1, chShared2),
			wantError:    true,
			errorSummary: "Duplicate Slack Channel",
			wantPath:     channelsPath.AtListIndex(1),
		},
		{
			name:         "duplicate shared channel with unknown workspace is invalid",
			channels:     list(chSharedUnknownWs1, chSharedUnknownWs2),
			wantError:    true,
			errorSummary: "Duplicate Slack Channel",
			wantPath:     channelsPath.AtListIndex(1),
		},
		{
			name:         "duplicate workspace channel is invalid",
			channels:     list(chWsA1, chWsA2),
			wantError:    true,
			errorSummary: "Duplicate Slack Channel",
			wantPath:     channelsPath.AtListIndex(1),
		},
		{
			name:         "unknown element before known invalid channel still reports invalid channel",
			channels:     list(unknownElement, sharedWithWs),
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
			wantPath:     channelsPath.AtListIndex(1).AtName("workspace"),
		},
		{
			name:         "unknown element after known invalid channel still reports invalid channel",
			channels:     list(sharedWithWs, unknownElement),
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
			wantPath:     channelsPath.AtListIndex(0).AtName("workspace"),
		},
		{
			name:         "unknown element between duplicate channels still reports duplicate",
			channels:     list(chShared1, unknownElement, chShared2),
			wantError:    true,
			errorSummary: "Duplicate Slack Channel",
			wantPath:     channelsPath.AtListIndex(2),
		},
		{
			name:      "same channel ID in different workspaces is valid",
			channels:  list(chWsA1, chWsB),
			wantError: false,
		},
		{
			name:      "same channel ID one shared and one in workspace is valid",
			channels:  list(chShared1, chWsA1),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildAlertValidatorConfig(ctx, t, nil, tt.channels, nil)
			req := resource.ValidateConfigRequest{Config: cfg}
			resp := &resource.ValidateConfigResponse{}
			alertSlackChannelsValidator{}.ValidateResource(ctx, req, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diags: %v)", got, tt.wantError, resp.Diagnostics)
			}
			assertAlertValidatorDiagnostics(t, resp.Diagnostics, tt.wantError, tt.errorSummary, tt.wantPath)
		})
	}
}
