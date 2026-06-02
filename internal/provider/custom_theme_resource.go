package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_custom_theme"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type (
	customThemeResource struct {
		client *models.ClientWithResponses
	}
	customThemeResourceModel struct {
		resource_custom_theme.CustomThemeModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*customThemeResource)(nil)
	_ resource.ResourceWithConfigure   = (*customThemeResource)(nil)
	_ resource.ResourceWithImportState = (*customThemeResource)(nil)
)

// NewCustomThemeResource creates a new custom theme resource instance.
func NewCustomThemeResource() resource.Resource {
	return &customThemeResource{}
}

// Configure adds the provider configured client to the resource.
func (r *customThemeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *customThemeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_theme"
}

func (r *customThemeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// hexColorPattern is the regex from the OpenAPI HexColor type.
// Accepts #RGB, #RRGGBB, or #RRGGBBAA.
var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func (r *customThemeResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_custom_theme.CustomThemeResourceSchema(ctx)

	// Add UseStateForUnknown to stable Computed-only fields so they don't
	// show as "(known after apply)" on every plan that modifies the resource.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}

	// Add hex color element validation to the colors.light and colors.dark
	// lists. The codegen validates list size but not element format; the
	// OpenAPI spec defines HexColor with a pattern that we replicate here.
	if colorsAttr, ok := s.Attributes["colors"].(schema.SingleNestedAttribute); ok {
		hexElementValidator := listvalidator.ValueStringsAre(
			stringvalidator.RegexMatches(hexColorPattern, "must be a hex color (#RGB, #RRGGBB, or #RRGGBBAA)"),
		)

		if dark, ok := colorsAttr.Attributes["dark"].(schema.ListAttribute); ok {
			dark.Validators = append(dark.Validators, hexElementValidator)
			colorsAttr.Attributes["dark"] = dark
		}
		if light, ok := colorsAttr.Attributes["light"].(schema.ListAttribute); ok {
			light.Validators = append(light.Validators, hexElementValidator)
			colorsAttr.Attributes["light"] = light
		}
		s.Attributes["colors"] = colorsAttr
	}

	s.Description = "Manages a custom color theme for Cloud Analytics reports. " +
		"Custom themes define light and dark mode color palettes that can be applied to reports."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *customThemeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customThemeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Convert model to API request type
	apiReq, reqDiags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(reqDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new custom theme via API
	themeResp, err := r.client.CreateCustomThemeWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Custom Theme",
			"Could not create custom theme, unexpected error: "+err.Error(),
		)
		return
	}

	if themeResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Custom Theme",
			fmt.Sprintf("Could not create custom theme, status: %d, body: %s", themeResp.StatusCode(), string(themeResp.Body)),
		)
		return
	}

	if themeResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Custom Theme",
			"Could not create custom theme, empty response",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from API response.
	resp.Diagnostics.Append(overlayCustomThemeComputedFields(ctx, themeResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customThemeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customThemeResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// Get refreshed custom theme from API
	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customThemeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customThemeResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Get the ID from the state
	var state customThemeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	themeID := state.Id.ValueString()

	// Convert model to API request type
	apiReq, reqDiags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(reqDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update custom theme via API
	updateResp, err := r.client.UpdateCustomThemeWithResponse(ctx, themeID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Custom Theme",
			"Could not update custom theme, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Custom Theme",
			fmt.Sprintf("Could not update custom theme, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Custom Theme",
			"Received empty response body",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from API response.
	resp.Diagnostics.Append(overlayCustomThemeComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customThemeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customThemeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// Delete custom theme via API
	deleteResp, err := r.client.DeleteCustomThemeWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Custom Theme",
			"Could not delete custom theme, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Custom Theme",
			fmt.Sprintf("Could not delete custom theme, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
