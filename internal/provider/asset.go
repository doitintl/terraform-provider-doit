// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_asset"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateState fetches the asset from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *assetResource) populateState(ctx context.Context, state *assetResourceModel) diag.Diagnostics {
	assetResp, err := r.client.GetAssetWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Asset", "Could not read asset ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if assetResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if assetResp.StatusCode() != 200 || assetResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Asset", fmt.Sprintf("Unexpected status: %d, body: %s", assetResp.StatusCode(), string(assetResp.Body))),
		}
	}

	return mapAssetToModel(ctx, assetResp.JSON200, state)
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

// overlayAssetComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Update.
//
// Asset is import-only (no Create). Only quantity is user-configurable;
// all other fields are Computed-only.
func overlayAssetComputedFields(ctx context.Context, apiResp *models.AssetItemDetailed, plan *assetResourceModel) diag.Diagnostics {
	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags := mapAssetToModel(ctx, apiResp, &resolved)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay.
	// Id: Computed-only, but safely preserved from the plan's prior state.
	// Quantity: Optional+Computed — resolve when Unknown.
	if plan.Quantity.IsUnknown() {
		plan.Quantity = resolved.Quantity
	}

	// All other fields are Computed-only — always from resolved.
	plan.Name = resolved.Name
	plan.Type = resolved.Type
	plan.Url = resolved.Url
	plan.CreateTime = resolved.CreateTime
	plan.Properties = resolved.Properties

	return diags
}
