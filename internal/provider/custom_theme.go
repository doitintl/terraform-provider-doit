// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_custom_theme"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// mapCustomThemeToModel maps the API response to the Terraform model.
// Used by Read and ImportState (and internally by the overlay as Phase 1).
func mapCustomThemeToModel(ctx context.Context, resp *models.CustomTheme, state *customThemeResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.PrimaryColor = types.StringValue(resp.PrimaryColor)

	if resp.CreateTime != nil {
		state.CreateTime = types.StringValue(resp.CreateTime.Format(time.RFC3339))
	} else {
		state.CreateTime = types.StringNull()
	}

	if resp.UpdateTime != nil {
		state.UpdateTime = types.StringValue(resp.UpdateTime.Format(time.RFC3339))
	} else {
		state.UpdateTime = types.StringNull()
	}

	// Map colors nested object
	colorsVal, colorsDiags := colorsToModel(ctx, resp.Colors)
	diags.Append(colorsDiags...)
	if diags.HasError() {
		return diags
	}
	state.Colors = colorsVal

	return diags
}

// overlayCustomThemeComputedFields uses the two-phase overlay pattern to
// reconcile the Terraform plan with the API response after Create/Update.
//
// CustomTheme has no Optional+Computed fields — all non-Required fields are
// Computed-only — so the overlay simply sets Computed fields from the
// resolved model while preserving the plan's Required fields.
func overlayCustomThemeComputedFields(ctx context.Context, apiResp *models.CustomTheme, plan *customThemeResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapCustomThemeToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields.
	plan.Id = resolved.Id
	plan.CreateTime = resolved.CreateTime
	plan.UpdateTime = resolved.UpdateTime

	// Name, PrimaryColor, Colors: Required — never touch.

	return diags
}

// colorsToModel converts an API ThemeColors response into the generated ColorsValue type.
func colorsToModel(ctx context.Context, tc models.ThemeColors) (resource_custom_theme.ColorsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Convert []HexColor → []attr.Value
	lightVals := make([]attr.Value, len(tc.Light))
	for i, c := range tc.Light {
		lightVals[i] = types.StringValue(c)
	}

	darkVals := make([]attr.Value, len(tc.Dark))
	for i, c := range tc.Dark {
		darkVals[i] = types.StringValue(c)
	}

	lightList, lightDiags := types.ListValue(types.StringType, lightVals)
	diags.Append(lightDiags...)
	darkList, darkDiags := types.ListValue(types.StringType, darkVals)
	diags.Append(darkDiags...)

	if diags.HasError() {
		return resource_custom_theme.NewColorsValueUnknown(), diags
	}

	colorsVal, colorsDiags := resource_custom_theme.NewColorsValue(
		resource_custom_theme.ColorsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"light": lightList,
			"dark":  darkList,
		},
	)
	diags.Append(colorsDiags...)

	return colorsVal, diags
}

// colorsFromPlan extracts the ThemeColors API model from the plan's ColorsValue.
func colorsFromPlan(ctx context.Context, colors resource_custom_theme.ColorsValue) (models.ThemeColors, diag.Diagnostics) {
	var diags diag.Diagnostics

	var light, dark []string
	diags.Append(colors.Light.ElementsAs(ctx, &light, false)...)
	diags.Append(colors.Dark.ElementsAs(ctx, &dark, false)...)
	if diags.HasError() {
		return models.ThemeColors{}, diags
	}

	lightHex := make([]models.HexColor, len(light))
	copy(lightHex, light)

	darkHex := make([]models.HexColor, len(dark))
	copy(darkHex, dark)

	return models.ThemeColors{
		Light: lightHex,
		Dark:  darkHex,
	}, diags
}

// colorsFromPlanPtr is like colorsFromPlan but returns a pointer for the
// Update request where colors is optional.
func colorsFromPlanPtr(ctx context.Context, colors resource_custom_theme.ColorsValue) (*models.ThemeColors, diag.Diagnostics) {
	tc, diags := colorsFromPlan(ctx, colors)
	if diags.HasError() {
		return nil, diags
	}
	return &tc, diags
}

// hexColorPtr returns a pointer to a HexColor value, for use in Update requests.
func hexColorPtr(v basetypes.StringValue) *models.HexColor {
	return new(v.ValueString())
}
