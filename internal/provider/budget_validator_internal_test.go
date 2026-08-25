package provider

import (
	"context"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestValidateBudgetStartPeriod(t *testing.T) {
	tests := []struct {
		name          string
		budgetType    string
		timeInterval  string
		startPeriod   time.Time
		expectedError bool
	}{
		{
			name:          "Fixed budget",
			budgetType:    "fixed",
			timeInterval:  "",
			startPeriod:   time.Now(),
			expectedError: false,
		},
		{
			name:          "Recurring Year Valid",
			budgetType:    "recurring",
			timeInterval:  "year",
			startPeriod:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Recurring Year Invalid",
			budgetType:    "recurring",
			timeInterval:  "year",
			startPeriod:   time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			expectedError: true,
		},
		{
			name:          "Recurring Month Valid",
			budgetType:    "recurring",
			timeInterval:  "month",
			startPeriod:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Recurring Month Invalid",
			budgetType:    "recurring",
			timeInterval:  "month",
			startPeriod:   time.Date(2025, 2, 1, 1, 0, 0, 0, time.UTC),
			expectedError: true,
		},
		{
			name:          "Recurring Quarter Valid",
			budgetType:    "recurring",
			timeInterval:  "quarter",
			startPeriod:   time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC), // Apr 1st is start of Q2
			expectedError: false,
		},
		{
			name:          "Recurring Quarter Invalid",
			budgetType:    "recurring",
			timeInterval:  "quarter",
			startPeriod:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), // Feb 1st is not start of a quarter
			expectedError: true,
		},
		{
			name:          "Recurring Week Valid (Monday)",
			budgetType:    "recurring",
			timeInterval:  "week",
			startPeriod:   time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), // Dec 1 2025 is Monday
			expectedError: false,
		},
		{
			name:          "Recurring Week Invalid (Tuesday)",
			budgetType:    "recurring",
			timeInterval:  "week",
			startPeriod:   time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC), // Dec 2 2025 is Tuesday
			expectedError: true,
		},
		{
			name:          "Recurring Day Valid",
			budgetType:    "recurring",
			timeInterval:  "day",
			startPeriod:   time.Date(2025, 12, 3, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Recurring Day Invalid",
			budgetType:    "recurring",
			timeInterval:  "day",
			startPeriod:   time.Date(2025, 12, 3, 0, 0, 1, 0, time.UTC),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBudgetStartPeriod(tt.budgetType, tt.timeInterval, tt.startPeriod.UnixMilli())
			if (err != nil) != tt.expectedError {
				t.Errorf("validateBudgetStartPeriod() error = %v, expectedError %v", err, tt.expectedError)
			}
		})
	}
}

func TestValidateBudgetTimeInterval(t *testing.T) {
	tests := []struct {
		name          string
		timeInterval  string
		expectedError bool
	}{
		{
			name:          "Valid day",
			timeInterval:  "day",
			expectedError: false,
		},
		{
			name:          "Valid week",
			timeInterval:  "week",
			expectedError: false,
		},
		{
			name:          "Valid month",
			timeInterval:  "month",
			expectedError: false,
		},
		{
			name:          "Valid quarter",
			timeInterval:  "quarter",
			expectedError: false,
		},
		{
			name:          "Valid year",
			timeInterval:  "year",
			expectedError: false,
		},
		{
			name:          "Invalid interval",
			timeInterval:  "decade",
			expectedError: true,
		},
		{
			name:          "Empty interval",
			timeInterval:  "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBudgetTimeInterval(tt.timeInterval)
			if (err != nil) != tt.expectedError {
				t.Errorf("validateBudgetTimeInterval() error = %v, expectedError %v", err, tt.expectedError)
			}
		})
	}
}

func TestValidateBudgetEndPeriod(t *testing.T) {
	tests := []struct {
		name          string
		endPeriod     int64
		expectedError bool
	}{
		{
			name:          "Valid End Period",
			endPeriod:     1600000000000,
			expectedError: false,
		},
		{
			name:          "Invalid Magic Value",
			endPeriod:     2678400000,
			expectedError: true,
		},
	}

	v := budgetEndPeriodValidator{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.Int64Request{
				ConfigValue: types.Int64Value(tt.endPeriod),
			}
			resp := &validator.Int64Response{}

			v.ValidateInt64(ctx, req, resp)

			if resp.Diagnostics.HasError() != tt.expectedError {
				t.Errorf("ValidateInt64() error = %v, expectedError %v", resp.Diagnostics.HasError(), tt.expectedError)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBudgetScopeNAValidator — unknown element handling
// ---------------------------------------------------------------------------

// TestBudgetScopeNAValidator_UnknownElement reproduces the "Value Conversion
// Error" crash when a budget scope's values list contains an unknown element
// from a cross-resource reference (e.g. values = [doit_allocation.xxx.id]).
func TestBudgetScopeNAValidator_UnknownElement(t *testing.T) {
	ctx := context.Background()

	scopeVal, sDiags := resource_budget.NewScopesValue(
		resource_budget.ScopesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"case_insensitive": types.BoolValue(false),
			"id":               types.StringValue("allocation_rule"),
			"include_null":     types.BoolValue(false),
			"inverse":          types.BoolNull(),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("allocation_rule"),
			"values": types.ListValueMust(types.StringType, []attr.Value{
				types.StringUnknown(), // simulates doit_allocation.xxx.id during plan
			}),
		},
	)
	if sDiags.HasError() {
		t.Fatalf("NewScopesValue: %v", sDiags)
	}

	valueLists := []types.List{scopeVal.Values}
	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("scopes"), valueLists, &diags)

	if diags.HasError() {
		t.Fatalf("warnNASentinels crashed with budget scope unknown element: %v", diags)
	}
}

type testSlackChannelConfig struct {
	id         attr.Value
	customerID attr.Value
	workspace  attr.Value
	shared     attr.Value
}

func buildBudgetSlackConfig(ctx context.Context, t *testing.T, channels []testSlackChannelConfig) tfsdk.Config {
	t.Helper()
	schema := resource_budget.BudgetResourceSchema(ctx)

	var slackList types.List
	if channels == nil {
		slackList = types.ListNull(resource_budget.RecipientsSlackChannelsValue{}.Type(ctx))
	} else {
		elems := make([]attr.Value, len(channels))
		for i, ch := range channels {
			id := ch.id
			if id == nil {
				id = types.StringValue("channel-123")
			}
			customerID := ch.customerID
			if customerID == nil {
				customerID = types.StringNull()
			}
			workspace := ch.workspace
			if workspace == nil {
				workspace = types.StringNull()
			}
			shared := ch.shared
			if shared == nil {
				shared = types.BoolNull()
			}

			val, diags := resource_budget.NewRecipientsSlackChannelsValue(
				resource_budget.RecipientsSlackChannelsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          id,
					"customer_id": customerID,
					"workspace":   workspace,
					"shared":      shared,
					"name":        types.StringNull(),
					"type":        types.StringNull(),
				},
			)
			if diags.HasError() {
				t.Fatalf("NewRecipientsSlackChannelsValue: %v", diags)
			}
			elems[i] = val
		}

		var listDiags diag.Diagnostics
		slackList, listDiags = types.ListValueFrom(
			ctx,
			resource_budget.RecipientsSlackChannelsValue{}.Type(ctx),
			elems,
		)
		if listDiags.HasError() {
			t.Fatalf("ListValueFrom: %v", listDiags)
		}
	}

	schemaType := schema.Type().TerraformType(ctx)
	objType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}

	attrValues := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrValues[name] = tftypes.NewValue(attrType, nil)
	}

	slackTFValue, err := slackList.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("ToTerraformValue: %v", err)
	}
	attrValues["recipients_slack_channels"] = slackTFValue

	rawValue := tftypes.NewValue(schemaType, attrValues)
	return tfsdk.Config{
		Schema: schema,
		Raw:    rawValue,
	}
}

func TestBudgetSlackChannelsValidator(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		channels      []testSlackChannelConfig
		expectedError bool
		errorCount    int
	}{
		{
			name:          "Null list passes",
			channels:      nil,
			expectedError: false,
		},
		{
			name:          "Empty list passes",
			channels:      []testSlackChannelConfig{},
			expectedError: false,
		},
		{
			name: "Shared is true without customer_id and workspace passes",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringNull(),
					workspace:  types.StringNull(),
					shared:     types.BoolValue(true),
				},
			},
			expectedError: false,
		},
		{
			name: "Shared is true with customer_id and workspace passes",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringValue("cust-1"),
					workspace:  types.StringValue("ws-1"),
					shared:     types.BoolValue(true),
				},
			},
			expectedError: false,
		},
		{
			name: "Shared is false with customer_id and workspace passes",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringValue("cust-1"),
					workspace:  types.StringValue("ws-1"),
					shared:     types.BoolValue(false),
				},
			},
			expectedError: false,
		},
		{
			name: "Shared is false missing customer_id fails",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringNull(),
					workspace:  types.StringValue("ws-1"),
					shared:     types.BoolValue(false),
				},
			},
			expectedError: true,
			errorCount:    1,
		},
		{
			name: "Shared is false missing workspace fails",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringValue("cust-1"),
					workspace:  types.StringNull(),
					shared:     types.BoolValue(false),
				},
			},
			expectedError: true,
			errorCount:    1,
		},
		{
			name: "Shared is false missing both customer_id and workspace fails",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringNull(),
					workspace:  types.StringNull(),
					shared:     types.BoolValue(false),
				},
			},
			expectedError: true,
			errorCount:    2,
		},
		{
			name: "Shared is false with empty strings fails",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringValue(""),
					workspace:  types.StringValue(""),
					shared:     types.BoolValue(false),
				},
			},
			expectedError: true,
			errorCount:    2,
		},
		{
			name: "Shared is null (omitted) missing customer_id and workspace fails",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringNull(),
					workspace:  types.StringNull(),
					shared:     types.BoolNull(),
				},
			},
			expectedError: true,
			errorCount:    2,
		},
		{
			name: "Shared is null (omitted) with customer_id and workspace passes",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringValue("cust-1"),
					workspace:  types.StringValue("ws-1"),
					shared:     types.BoolNull(),
				},
			},
			expectedError: false,
		},
		{
			name: "Shared is unknown with null customer_id and workspace passes (deferred)",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringNull(),
					workspace:  types.StringNull(),
					shared:     types.BoolUnknown(),
				},
			},
			expectedError: false,
		},
		{
			name: "Customer_id and workspace unknown pass (deferred)",
			channels: []testSlackChannelConfig{
				{
					id:         types.StringValue("C12345"),
					customerID: types.StringUnknown(),
					workspace:  types.StringUnknown(),
					shared:     types.BoolValue(false),
				},
			},
			expectedError: false,
		},
	}

	v := budgetSlackChannelsValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildBudgetSlackConfig(ctx, t, tt.channels)
			req := resource.ValidateConfigRequest{Config: config}
			resp := &resource.ValidateConfigResponse{}

			v.ValidateResource(ctx, req, resp)

			if resp.Diagnostics.HasError() != tt.expectedError {
				t.Errorf("ValidateResource() hasError = %v, expectedError %v (diagnostics: %v)",
					resp.Diagnostics.HasError(), tt.expectedError, resp.Diagnostics)
			}
			if tt.errorCount > 0 && resp.Diagnostics.ErrorsCount() != tt.errorCount {
				t.Errorf("ValidateResource() errorCount = %d, expected %d (diagnostics: %v)",
					resp.Diagnostics.ErrorsCount(), tt.errorCount, resp.Diagnostics)
			}
		})
	}
}
