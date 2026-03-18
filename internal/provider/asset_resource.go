package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_asset"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	assetResource struct {
		client *models.ClientWithResponses
	}
	assetResourceModel struct {
		resource_asset.AssetModel
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*assetResource)(nil)
	_ resource.ResourceWithConfigure   = (*assetResource)(nil)
	_ resource.ResourceWithImportState = (*assetResource)(nil)
)

// NewAssetResource creates a new asset resource instance.
func NewAssetResource() resource.Resource {
	return &assetResource{}
}

// Configure adds the provider configured client to the resource.
func (r *assetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *assetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset"
}

func (r *assetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *assetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_asset.AssetResourceSchema(ctx)

	// Override id to be Required (user must specify the asset to manage).
	// This is the only full override needed — import-only resources require
	// the user to specify the identifier in their config.
	s.Attributes["id"] = schema.StringAttribute{
		Required:            true,
		Description:         "The unique id of an asset.",
		MarkdownDescription: "The unique id of an asset.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}

	// Add UseStateForUnknown to computed-only fields so they don't show as
	// "(known after apply)" when an update is triggered by a quantity change.
	// We modify the generated attributes in-place to preserve their descriptions,
	// types, validators, etc.
	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["name"] = attr
	}
	if attr, ok := s.Attributes["type"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["type"] = attr
	}
	if attr, ok := s.Attributes["url"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["url"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.Int64Attribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, int64planmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}

	s.Description = "Manages a DoiT asset (e.g., G Suite/Google Workspace licenses). " +
		"This resource is import-only: assets must first be created through the DoiT Console " +
		"and then imported into Terraform using `terraform import`. " +
		"Destroying this resource only removes it from Terraform state; the asset continues to exist in DoiT."
	s.MarkdownDescription = s.Description

	resp.Schema = s
}

func (r *assetResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"Asset Creation Not Supported",
		"The doit_asset resource does not support creation. "+
			"Assets must be created through the DoiT Console and then imported into Terraform.\n\n"+
			"To import an existing asset, use:\n"+
			"  terraform import doit_asset.<name> <asset-id>",
	)
}

func (r *assetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state assetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed asset value from API
	assetResp, err := r.client.GetAssetWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Asset",
			"Could not read asset ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle externally deleted resource
	if assetResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if assetResp.StatusCode() != 200 || assetResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Asset",
			fmt.Sprintf("Unexpected status: %d, body: %s", assetResp.StatusCode(), string(assetResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapAssetToModel(ctx, assetResp.JSON200, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan assetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the ID from state
	var state assetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	assetID := state.Id.ValueString()

	// Build the update request body
	updateBody := models.IdOfAssetJSONRequestBody{}
	if !plan.Quantity.IsNull() && !plan.Quantity.IsUnknown() {
		q := plan.Quantity.ValueInt64()
		updateBody.Quantity = &q
	}

	// Call PATCH
	updateResp, err := r.client.IdOfAssetWithResponse(ctx, assetID, updateBody)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Asset",
			"Could not update asset, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Asset",
			fmt.Sprintf("Could not update asset, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	// The PATCH response doesn't return the full AssetItem, so GET the updated state
	assetResp, err := r.client.GetAssetWithResponse(ctx, assetID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Asset After Update",
			"Could not read asset after update: "+err.Error(),
		)
		return
	}

	if assetResp.StatusCode() != 200 || assetResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Asset After Update",
			fmt.Sprintf("Asset not found after update, status: %d, body: %s", assetResp.StatusCode(), string(assetResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapAssetToModel(ctx, assetResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state assetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No-op delete: the DoiT API does not support deleting assets.
	// We simply remove the resource from Terraform state.
	resp.Diagnostics.AddWarning(
		"Asset Not Deleted from DoiT API",
		"doit_asset does not support deletion via the API. "+
			"The asset has been removed from Terraform state but continues to exist in DoiT.",
	)
}

// mapAssetToModel maps the API response to the Terraform resource model.
func mapAssetToModel(ctx context.Context, asset *models.AssetItemDetailed, state *assetResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(asset.Id)
	state.Name = types.StringPointerValue(asset.Name)
	state.Type = types.StringPointerValue(asset.Type)
	state.Url = types.StringPointerValue(asset.Url)
	state.Quantity = types.Int64PointerValue(asset.Quantity)
	state.CreateTime = types.Int64PointerValue(asset.CreateTime)

	if asset.Properties == nil {
		state.Properties = resource_asset.NewPropertiesValueNull()
		return diags
	}

	props := asset.Properties

	subscriptionVal, d := mapSubscriptionToValue(ctx, props.Subscription)
	diags.Append(d...)

	propsVal, d := resource_asset.NewPropertiesValue(
		resource_asset.PropertiesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"customer_domain": types.StringPointerValue(props.CustomerDomain),
			"customer_id":     types.StringPointerValue(props.CustomerID),
			"reseller":        types.StringPointerValue(props.Reseller),
			"subscription":    subscriptionVal,
		},
	)
	diags.Append(d...)

	state.Properties = propsVal

	return diags
}

func mapSubscriptionToValue(ctx context.Context, sub *models.Subscription) (resource_asset.SubscriptionValue, diag.Diagnostics) {
	if sub == nil {
		return resource_asset.NewSubscriptionValueNull(), nil
	}

	var diags diag.Diagnostics

	planVal, d := mapPlanToValue(ctx, sub.Plan)
	diags.Append(d...)

	renewalVal, d := mapRenewalSettingsToValue(ctx, sub.RenewalSettings)
	diags.Append(d...)

	seatsVal, d := mapSeatsToValue(ctx, sub.Seats)
	diags.Append(d...)

	val, d := resource_asset.NewSubscriptionValue(
		resource_asset.SubscriptionValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"billing_method":    types.StringPointerValue(sub.BillingMethod),
			"creation_time":     types.Int64PointerValue(sub.CreationTime),
			"id":                types.StringPointerValue(sub.Id),
			"plan":              planVal,
			"purchase_order_id": types.StringPointerValue(sub.PurchaseOrderID),
			"renewal_settings":  renewalVal,
			"resource_uiurl":    types.StringPointerValue(sub.ResourceUIURL),
			"seats":             seatsVal,
			"sku_id":            types.StringPointerValue(sub.SkuID),
			"sku_name":          types.StringPointerValue(sub.SkuName),
			"status":            types.StringPointerValue(sub.Status),
		},
	)
	diags.Append(d...)

	return val, diags
}

func mapPlanToValue(ctx context.Context, plan *models.SubscriptionPlan) (resource_asset.PlanValue, diag.Diagnostics) {
	if plan == nil {
		return resource_asset.NewPlanValueNull(), nil
	}

	var diags diag.Diagnostics

	commitmentVal, d := mapCommitmentIntervalToValue(ctx, plan.CommitmentInterval)
	diags.Append(d...)

	val, d := resource_asset.NewPlanValue(
		resource_asset.PlanValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"commitment_interval": commitmentVal,
			"is_commitment_plan":  types.BoolPointerValue(plan.IsCommitmentPlan),
			"plan_name":           types.StringPointerValue(plan.PlanName),
		},
	)
	diags.Append(d...)

	return val, diags
}

func mapCommitmentIntervalToValue(ctx context.Context, ci *models.SubscriptionPlanCommitmentInterval) (resource_asset.CommitmentIntervalValue, diag.Diagnostics) {
	if ci == nil {
		return resource_asset.NewCommitmentIntervalValueNull(), nil
	}

	val, d := resource_asset.NewCommitmentIntervalValue(
		resource_asset.CommitmentIntervalValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"end_time":   types.Int64PointerValue(ci.EndTime),
			"start_time": types.Int64PointerValue(ci.StartTime),
		},
	)

	return val, d
}

func mapRenewalSettingsToValue(ctx context.Context, rs *models.RenewalSettings) (resource_asset.RenewalSettingsValue, diag.Diagnostics) {
	if rs == nil {
		return resource_asset.NewRenewalSettingsValueNull(), nil
	}

	val, d := resource_asset.NewRenewalSettingsValue(
		resource_asset.RenewalSettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"renewal_type": types.StringPointerValue(rs.RenewalType),
		},
	)

	return val, d
}

func mapSeatsToValue(ctx context.Context, seats *models.Seats) (resource_asset.SeatsValue, diag.Diagnostics) {
	if seats == nil {
		return resource_asset.NewSeatsValueNull(), nil
	}

	val, d := resource_asset.NewSeatsValue(
		resource_asset.SeatsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"licensed_number_of_seats": types.Int64PointerValue(seats.LicensedNumberOfSeats),
			"maximum_number_of_seats":  types.Int64PointerValue(seats.MaximumNumberOfSeats),
			"number_of_seats":          types.Int64PointerValue(seats.NumberOfSeats),
		},
	)

	return val, d
}
