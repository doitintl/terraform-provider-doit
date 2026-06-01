package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_active_theme"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	activeThemeResource struct {
		client *models.ClientWithResponses
	}
	activeThemeResourceModel struct {
		resource_active_theme.ActiveThemeModel
		Id       types.String   `tfsdk:"id"`
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*activeThemeResource)(nil)
	_ resource.ResourceWithConfigure   = (*activeThemeResource)(nil)
	_ resource.ResourceWithImportState = (*activeThemeResource)(nil)
)

// NewActiveThemeResource creates a new active theme resource instance.
func NewActiveThemeResource() resource.Resource {
	return &activeThemeResource{}
}

// Configure adds the provider configured client to the resource.
func (r *activeThemeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *activeThemeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_active_theme"
}

func (r *activeThemeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *activeThemeResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_active_theme.ActiveThemeResourceSchema(ctx)

	// Add synthetic id attribute — this singleton resource uses a fixed ID.
	s.Attributes["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "Synthetic resource identifier. Always \"active-theme\".",
		MarkdownDescription: "Synthetic resource identifier. Always `\"active-theme\"`.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}

	s.Description = "Manages the active color theme for the current account. " +
		"This is a singleton resource — there is exactly one active theme per account. " +
		"Destroying this resource resets the theme to the built-in default."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *activeThemeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan activeThemeResourceModel

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

	apiReq := plan.toUpdateRequest()

	themeResp, err := r.client.SetActiveThemeWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Active Theme",
			"Could not set active theme, unexpected error: "+err.Error(),
		)
		return
	}

	if themeResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Creating Active Theme",
			fmt.Sprintf("Could not set active theme, status: %d, body: %s", themeResp.StatusCode(), string(themeResp.Body)),
		)
		return
	}

	if themeResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Active Theme",
			"Could not set active theme, empty response",
		)
		return
	}

	overlayActiveThemeComputedFields(themeResp.JSON200, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *activeThemeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state activeThemeResourceModel

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

	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *activeThemeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan activeThemeResourceModel

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

	apiReq := plan.toUpdateRequest()

	updateResp, err := r.client.SetActiveThemeWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Active Theme",
			"Could not update active theme, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Active Theme",
			fmt.Sprintf("Could not update active theme, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Active Theme",
			"Received empty response body",
		)
		return
	}

	overlayActiveThemeComputedFields(updateResp.JSON200, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *activeThemeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state activeThemeResourceModel

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

	// Reset to built-in default — there is no DELETE endpoint.
	resetReq := models.SetActiveThemeRequest{
		ThemeId: "default",
	}

	deleteResp, err := r.client.SetActiveThemeWithResponse(ctx, resetReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Active Theme",
			"Could not reset active theme to default, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success — nothing to reset.
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Active Theme",
			fmt.Sprintf("Could not reset active theme to default, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
