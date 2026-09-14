package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

func TestAlertRecipientsValidator(t *testing.T) {
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

	tests := []struct {
		name          string
		recipients    attr.Value
		slackChannels attr.Value
		wantError     bool
		errorSummary  string
	}{
		{
			name:          "both omitted (null) is valid (defaults to owner)",
			recipients:    recipientsNull,
			slackChannels: slackNull,
			wantError:     false,
		},
		{
			name:          "recipients populated, slack null is valid",
			recipients:    recipientsPopulated,
			slackChannels: slackNull,
			wantError:     false,
		},
		{
			name:          "recipients populated, slack empty is valid",
			recipients:    recipientsPopulated,
			slackChannels: slackEmpty,
			wantError:     false,
		},
		{
			name:          "recipients empty, slack populated is valid",
			recipients:    recipientsEmpty,
			slackChannels: slackPopulated,
			wantError:     false,
		},
		{
			name:          "recipients empty, slack null is invalid",
			recipients:    recipientsEmpty,
			slackChannels: slackNull,
			wantError:     true,
			errorSummary:  "At Least One Recipient Required",
		},
		{
			name:          "recipients empty, slack empty is invalid",
			recipients:    recipientsEmpty,
			slackChannels: slackEmpty,
			wantError:     true,
			errorSummary:  "At Least One Recipient Required",
		},
		{
			name:          "recipients null, slack empty is invalid",
			recipients:    recipientsNull,
			slackChannels: slackEmpty,
			wantError:     true,
			errorSummary:  "At Least One Destination Required",
		},
		{
			name:          "recipients unknown is deferred",
			recipients:    recipientsUnknown,
			slackChannels: slackEmpty,
			wantError:     false,
		},
		{
			name:          "slack unknown is deferred",
			recipients:    recipientsEmpty,
			slackChannels: slackUnknown,
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildAlertValidatorConfig(ctx, t, tt.recipients, tt.slackChannels, nil)
			req := resource.ValidateConfigRequest{Config: cfg}
			resp := &resource.ValidateConfigResponse{}
			alertRecipientsValidator{}.ValidateResource(ctx, req, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diags: %v)", got, tt.wantError, resp.Diagnostics)
			}
			if tt.wantError && tt.errorSummary != "" {
				if summary := resp.Diagnostics.Errors()[0].Summary(); summary != tt.errorSummary {
					t.Errorf("summary = %q, want %q", summary, tt.errorSummary)
				}
			}
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

	rangeNull := resource_alert.NewIgnoreValuesRangeValueNull()
	rangeUnknown := resource_alert.NewIgnoreValuesRangeValueUnknown()

	tests := []struct {
		name              string
		condition         attr.Value
		ignoreValuesRange attr.Value
		wantError         bool
		errorSummary      string
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
		},
		{
			name:              "range configured with condition value is invalid",
			condition:         types.StringValue("value"),
			ignoreValuesRange: validRange,
			wantError:         true,
			errorSummary:      "Invalid Ignore Values Range Condition",
		},
		{
			name:              "range configured with condition forecasted-value is invalid",
			condition:         types.StringValue("forecasted-value"),
			ignoreValuesRange: validRange,
			wantError:         true,
			errorSummary:      "Invalid Ignore Values Range Condition",
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
			if tt.wantError && tt.errorSummary != "" {
				if summary := resp.Diagnostics.Errors()[0].Summary(); summary != tt.errorSummary {
					t.Errorf("summary = %q, want %q", summary, tt.errorSummary)
				}
			}
		})
	}
}

func makeSlackChannel(
	ctx context.Context,
	t *testing.T,
	id string,
	shared attr.Value,
	workspace attr.Value,
) resource_alert.RecipientsSlackChannelsValue {
	t.Helper()
	v, d := resource_alert.NewRecipientsSlackChannelsValue(
		resource_alert.RecipientsSlackChannelsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"customer_id": types.StringNull(),
			"id":          types.StringValue(id),
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

	tests := []struct {
		name         string
		channels     func() types.List
		wantError    bool
		errorSummary string
	}{
		{
			name: "shared true without workspace is valid",
			channels: func() types.List {
				ch := makeSlackChannel(ctx, t, "C123", types.BoolValue(true), types.StringNull())
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError: false,
		},
		{
			name: "shared true with workspace is invalid",
			channels: func() types.List {
				ch := makeSlackChannel(ctx, t, "C123", types.BoolValue(true), types.StringValue("my-workspace"))
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
		},
		{
			name: "shared false with workspace is valid",
			channels: func() types.List {
				ch := makeSlackChannel(ctx, t, "C123", types.BoolValue(false), types.StringValue("my-workspace"))
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError: false,
		},
		{
			name: "shared false without workspace is invalid",
			channels: func() types.List {
				ch := makeSlackChannel(ctx, t, "C123", types.BoolValue(false), types.StringNull())
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
		},
		{
			name: "shared null (omitted) with workspace is valid",
			channels: func() types.List {
				ch := makeSlackChannel(ctx, t, "C123", types.BoolNull(), types.StringValue("my-workspace"))
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError: false,
		},
		{
			name: "shared null (omitted) without workspace is invalid",
			channels: func() types.List {
				ch := makeSlackChannel(ctx, t, "C123", types.BoolNull(), types.StringNull())
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError:    true,
			errorSummary: "Invalid Slack Channel Shape",
		},
		{
			name: "duplicate shared channel is invalid",
			channels: func() types.List {
				ch1 := makeSlackChannel(ctx, t, "C123", types.BoolValue(true), types.StringNull())
				ch2 := makeSlackChannel(ctx, t, "C123", types.BoolValue(true), types.StringNull())
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch1, ch2})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError:    true,
			errorSummary: "Duplicate Slack Channel",
		},
		{
			name: "duplicate workspace channel is invalid",
			channels: func() types.List {
				ch1 := makeSlackChannel(ctx, t, "C123", types.BoolValue(false), types.StringValue("workspace-a"))
				ch2 := makeSlackChannel(ctx, t, "C123", types.BoolValue(false), types.StringValue("workspace-a"))
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch1, ch2})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError:    true,
			errorSummary: "Duplicate Slack Channel",
		},
		{
			name: "same channel ID in different workspaces is valid",
			channels: func() types.List {
				ch1 := makeSlackChannel(ctx, t, "C123", types.BoolValue(false), types.StringValue("workspace-a"))
				ch2 := makeSlackChannel(ctx, t, "C123", types.BoolValue(false), types.StringValue("workspace-b"))
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch1, ch2})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError: false,
		},
		{
			name: "same channel ID one shared and one in workspace is valid",
			channels: func() types.List {
				ch1 := makeSlackChannel(ctx, t, "C123", types.BoolValue(true), types.StringNull())
				ch2 := makeSlackChannel(ctx, t, "C123", types.BoolValue(false), types.StringValue("workspace-a"))
				l, d := types.ListValueFrom(ctx, resource_alert.RecipientsSlackChannelsValue{}.Type(ctx), []resource_alert.RecipientsSlackChannelsValue{ch1, ch2})
				if d.HasError() {
					t.Fatalf("ListValueFrom: %v", d)
				}
				return l
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildAlertValidatorConfig(ctx, t, nil, tt.channels(), nil)
			req := resource.ValidateConfigRequest{Config: cfg}
			resp := &resource.ValidateConfigResponse{}
			alertSlackChannelsValidator{}.ValidateResource(ctx, req, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diags: %v)", got, tt.wantError, resp.Diagnostics)
			}
			if tt.wantError && tt.errorSummary != "" {
				if summary := resp.Diagnostics.Errors()[0].Summary(); summary != tt.errorSummary {
					t.Errorf("summary = %q, want %q", summary, tt.errorSummary)
				}
			}
		})
	}
}
