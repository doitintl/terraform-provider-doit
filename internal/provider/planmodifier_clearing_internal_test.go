package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The clearing modifiers act only when config is null and a stored value exists.
// They must leave the plan alone on create (no stored value), when config holds a
// value, and when config is not known yet. In particular a plan that is unknown
// because another attribute changed must stay unknown when the stored value is
// null: there is nothing to clear and the overlay resolves it from the API.
func TestClearingPlanModifiers(t *testing.T) {
	ctx := t.Context()

	type scenario struct {
		name              string
		config, state, in attr.Value
		wantCleared       bool
	}

	scenarios := func(known, unknown, null attr.Value) []scenario {
		return []scenario{
			{name: "config null, stored value: clears", config: null, state: known, in: unknown, wantCleared: true},
			{name: "config null, stored value, plan holds it: clears", config: null, state: known, in: known, wantCleared: true},
			{name: "create: nothing stored", config: null, state: null, in: unknown},
			{name: "config set", config: known, state: known, in: known},
			{name: "config unknown", config: unknown, state: known, in: unknown},
		}
	}

	t.Run("string", func(t *testing.T) {
		for _, m := range []struct {
			name    string
			mod     planmodifier.String
			cleared types.String
		}{
			{"useEmptyForUnknownWhenConfigNull", useEmptyForUnknownWhenConfigNull(), types.StringValue("")},
			{"useNullForUnknownStringWhenConfigNull", useNullForUnknownStringWhenConfigNull(), types.StringNull()},
		} {
			for _, s := range scenarios(types.StringValue("x"), types.StringUnknown(), types.StringNull()) {
				t.Run(m.name+"/"+s.name, func(t *testing.T) {
					resp := &planmodifier.StringResponse{PlanValue: s.in.(types.String)}
					m.mod.PlanModifyString(ctx, planmodifier.StringRequest{
						ConfigValue: s.config.(types.String), StateValue: s.state.(types.String), PlanValue: s.in.(types.String),
					}, resp)
					want := s.in
					if s.wantCleared {
						want = m.cleared
					}
					if !resp.PlanValue.Equal(want) {
						t.Errorf("plan = %s, want %s", resp.PlanValue, want)
					}
				})
			}
		}
	})

	t.Run("bool", func(t *testing.T) {
		mod := useEmptyForUnknownBoolWhenConfigNull()
		for _, s := range scenarios(types.BoolValue(true), types.BoolUnknown(), types.BoolNull()) {
			t.Run(s.name, func(t *testing.T) {
				resp := &planmodifier.BoolResponse{PlanValue: s.in.(types.Bool)}
				mod.PlanModifyBool(ctx, planmodifier.BoolRequest{
					ConfigValue: s.config.(types.Bool), StateValue: s.state.(types.Bool), PlanValue: s.in.(types.Bool),
				}, resp)
				want := s.in
				if s.wantCleared {
					want = types.BoolValue(false)
				}
				if !resp.PlanValue.Equal(want) {
					t.Errorf("plan = %s, want %s", resp.PlanValue, want)
				}
			})
		}
	})

	t.Run("list", func(t *testing.T) {
		known := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("x")})
		unknown, null := types.ListUnknown(types.StringType), types.ListNull(types.StringType)
		for _, m := range []struct {
			name    string
			mod     planmodifier.List
			cleared types.List
		}{
			{"useNullForUnknownListWhenConfigNull", useNullForUnknownListWhenConfigNull(), types.ListValueMust(types.StringType, []attr.Value{})},
			{"useNullForUnknownListNullWhenConfigNull", useNullForUnknownListNullWhenConfigNull(types.StringType), null},
		} {
			for _, s := range scenarios(known, unknown, null) {
				t.Run(m.name+"/"+s.name, func(t *testing.T) {
					resp := &planmodifier.ListResponse{PlanValue: s.in.(types.List)}
					m.mod.PlanModifyList(ctx, planmodifier.ListRequest{
						ConfigValue: s.config.(types.List), StateValue: s.state.(types.List), PlanValue: s.in.(types.List),
					}, resp)
					want := s.in
					if s.wantCleared {
						want = m.cleared
					}
					if !resp.PlanValue.Equal(want) {
						t.Errorf("plan = %s, want %s", resp.PlanValue, want)
					}
				})
			}
		}
	})
}
