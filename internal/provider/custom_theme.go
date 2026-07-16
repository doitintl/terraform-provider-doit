// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_custom_theme"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// populateState fetches the custom theme from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *customThemeResource) populateState(ctx context.Context, state *customThemeResourceModel) diag.Diagnostics {
	themeResp, err := r.client.GetCustomThemeWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Custom Theme", "Could not read custom theme ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if themeResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if themeResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Custom Theme", fmt.Sprintf("Unexpected status code %d for custom theme ID %s: %s", themeResp.StatusCode(), state.Id.ValueString(), string(themeResp.Body))),
		}
	}

	if themeResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Custom Theme", "Received empty response body for custom theme ID "+state.Id.ValueString()),
		}
	}

	return mapCustomThemeToModel(ctx, themeResp.JSON200, state)
}

// mapCustomThemeToModel maps the API response to the Terraform model.
// Used by populateState (Read/ImportState) and as Phase 1 of overlay.
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

// toCreateRequest converts the TF model to a CreateCustomThemeRequest.
func (plan *customThemeResourceModel) toCreateRequest(ctx context.Context) (models.CreateCustomThemeRequest, diag.Diagnostics) {
	colors, diags := colorsFromPlan(ctx, plan.Colors)
	if diags.HasError() {
		return models.CreateCustomThemeRequest{}, diags
	}

	req := models.CreateCustomThemeRequest{
		Name:         plan.Name.ValueString(),
		PrimaryColor: plan.PrimaryColor.ValueString(),
		Colors:       colors,
	}

	return req, diags
}

// toUpdateRequest converts the TF model to an UpdateCustomThemeRequest.
// All fields are Required in the schema, so they will always be present.
// We use pointers because UpdateCustomThemeRequest uses pointer types
// for PATCH semantics, but we always send all fields.
func (plan *customThemeResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateCustomThemeRequest, diag.Diagnostics) {
	colorsPtr, diags := colorsFromPlanPtr(ctx, plan.Colors)
	if diags.HasError() {
		return models.UpdateCustomThemeRequest{}, diags
	}

	req := models.UpdateCustomThemeRequest{
		Name:         pointerToNullable(new(plan.Name.ValueString())),
		PrimaryColor: hexColorPtr(plan.PrimaryColor),
		Colors:       colorsPtr,
	}

	return req, diags
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

	tc := models.ThemeColors{
		Light: lightHex,
		Dark:  darkHex,
	}

	return tc, diags
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
