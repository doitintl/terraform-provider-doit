package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_asset"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*assetDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*assetDataSource)(nil)

func NewAssetDataSource() datasource.DataSource {
	return &assetDataSource{}
}

type assetDataSource struct {
	client *models.ClientWithResponses
}

type assetDataSourceModel struct {
	datasource_asset.AssetModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (ds *assetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset"
}

func (ds *assetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	ds.client = client
}

func (ds *assetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_asset.AssetDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (ds *assetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data assetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// If ID is unknown (depends on a resource not yet created), set all computed
	// attributes to unknown so consumers don't treat null as a real value during planning.
	if data.Id.IsUnknown() {
		data.Name = types.StringUnknown()
		data.Type = types.StringUnknown()
		data.Url = types.StringUnknown()
		data.Quantity = types.Int64Unknown()
		data.CreateTime = types.Int64Unknown()
		data.Properties = datasource_asset.NewPropertiesValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	id := data.Id.ValueString()
	assetResp, err := ds.client.GetAssetWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading asset", err.Error())
		return
	}
	if assetResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Asset not found", fmt.Sprintf("Asset with ID %s not found", id))
		return
	}
	if assetResp.StatusCode() != 200 || assetResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading asset",
			fmt.Sprintf("Unexpected status: %d, body: %s", assetResp.StatusCode(), string(assetResp.Body)),
		)
		return
	}

	asset := assetResp.JSON200

	data.Id = types.StringPointerValue(asset.Id)
	data.Name = types.StringPointerValue(asset.Name)
	data.Type = types.StringPointerValue(asset.Type)
	data.Url = types.StringPointerValue(asset.Url)
	data.Quantity = types.Int64PointerValue(asset.Quantity)
	data.CreateTime = types.Int64PointerValue(asset.CreateTime)

	resp.Diagnostics.Append(mapAssetPropertiesToDataSource(ctx, asset, &data.AssetModel)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAssetPropertiesToDataSource maps the API properties to the data source model.
func mapAssetPropertiesToDataSource(ctx context.Context, asset *models.AssetItemDetailed, state *datasource_asset.AssetModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if asset.Properties == nil {
		state.Properties = datasource_asset.NewPropertiesValueNull()
		return diags
	}

	props := asset.Properties

	subscriptionVal, d := mapSubscriptionToDataSource(ctx, props.Subscription)
	diags.Append(d...)

	propsVal, d := datasource_asset.NewPropertiesValue(
		datasource_asset.PropertiesValue{}.AttributeTypes(ctx),
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

func mapSubscriptionToDataSource(ctx context.Context, sub *models.Subscription) (datasource_asset.SubscriptionValue, diag.Diagnostics) {
	if sub == nil {
		return datasource_asset.NewSubscriptionValueNull(), nil
	}

	var diags diag.Diagnostics

	planVal, d := mapPlanToDataSource(ctx, sub.Plan)
	diags.Append(d...)

	renewalVal, d := mapRenewalSettingsToDataSource(ctx, sub.RenewalSettings)
	diags.Append(d...)

	seatsVal, d := mapSeatsToDataSource(ctx, sub.Seats)
	diags.Append(d...)

	val, d := datasource_asset.NewSubscriptionValue(
		datasource_asset.SubscriptionValue{}.AttributeTypes(ctx),
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

func mapPlanToDataSource(ctx context.Context, plan *models.SubscriptionPlan) (datasource_asset.PlanValue, diag.Diagnostics) {
	if plan == nil {
		return datasource_asset.NewPlanValueNull(), nil
	}

	var diags diag.Diagnostics

	commitmentVal, d := mapCommitmentIntervalToDataSource(ctx, plan.CommitmentInterval)
	diags.Append(d...)

	val, d := datasource_asset.NewPlanValue(
		datasource_asset.PlanValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"commitment_interval": commitmentVal,
			"is_commitment_plan":  types.BoolPointerValue(plan.IsCommitmentPlan),
			"plan_name":           types.StringPointerValue(plan.PlanName),
		},
	)
	diags.Append(d...)

	return val, diags
}

func mapCommitmentIntervalToDataSource(ctx context.Context, ci *models.SubscriptionPlanCommitmentInterval) (datasource_asset.CommitmentIntervalValue, diag.Diagnostics) {
	if ci == nil {
		return datasource_asset.NewCommitmentIntervalValueNull(), nil
	}

	val, d := datasource_asset.NewCommitmentIntervalValue(
		datasource_asset.CommitmentIntervalValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"end_time":   types.Int64PointerValue(ci.EndTime),
			"start_time": types.Int64PointerValue(ci.StartTime),
		},
	)

	return val, d
}

func mapRenewalSettingsToDataSource(ctx context.Context, rs *models.RenewalSettings) (datasource_asset.RenewalSettingsValue, diag.Diagnostics) {
	if rs == nil {
		return datasource_asset.NewRenewalSettingsValueNull(), nil
	}

	val, d := datasource_asset.NewRenewalSettingsValue(
		datasource_asset.RenewalSettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"renewal_type": types.StringPointerValue(rs.RenewalType),
		},
	)

	return val, d
}

func mapSeatsToDataSource(ctx context.Context, seats *models.Seats) (datasource_asset.SeatsValue, diag.Diagnostics) {
	if seats == nil {
		return datasource_asset.NewSeatsValueNull(), nil
	}

	val, d := datasource_asset.NewSeatsValue(
		datasource_asset.SeatsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"licensed_number_of_seats": types.Int64PointerValue(seats.LicensedNumberOfSeats),
			"maximum_number_of_seats":  types.Int64PointerValue(seats.MaximumNumberOfSeats),
			"number_of_seats":          types.Int64PointerValue(seats.NumberOfSeats),
		},
	)

	return val, d
}
