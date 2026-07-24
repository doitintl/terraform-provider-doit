package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestUseRuleIdFromStateWhenConfigNullModifier(t *testing.T) {
	ctx := context.Background()
	mod := useRuleIdFromStateWhenConfigNull()

	tests := []struct {
		name          string
		configVal     types.String
		stateVal      types.String
		expectedPlan  types.String
		expectPlanSet bool
	}{
		{
			name:          "config null, state has value -> preserves state value",
			configVal:     types.StringNull(),
			stateVal:      types.StringValue("alloc-123"),
			expectedPlan:  types.StringValue("alloc-123"),
			expectPlanSet: true,
		},
		{
			name:          "config set -> no change to plan value",
			configVal:     types.StringValue("alloc-custom"),
			stateVal:      types.StringValue("alloc-123"),
			expectPlanSet: false,
		},
		{
			name:          "config null, state null -> no change to plan value",
			configVal:     types.StringNull(),
			stateVal:      types.StringNull(),
			expectPlanSet: false,
		},
		{
			name:          "config null, state unknown -> no change to plan value",
			configVal:     types.StringNull(),
			stateVal:      types.StringUnknown(),
			expectPlanSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				ConfigValue: tt.configVal,
				StateValue:  tt.stateVal,
			}
			var resp planmodifier.StringResponse

			mod.PlanModifyString(ctx, req, &resp)

			if tt.expectPlanSet {
				if resp.PlanValue != tt.expectedPlan {
					t.Errorf("expected PlanValue %v, got %v", tt.expectedPlan, resp.PlanValue)
				}
			} else if !resp.PlanValue.IsUnknown() && !resp.PlanValue.IsNull() {
				t.Errorf("expected PlanValue to remain unset/unknown, got %v", resp.PlanValue)
			}
		})
	}
}
