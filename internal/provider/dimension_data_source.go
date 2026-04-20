package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_dimension"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*dimensionDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*dimensionDataSource)(nil)

func NewDimensionDataSource() datasource.DataSource {
	return &dimensionDataSource{}
}

type dimensionDataSource struct {
	client *models.ClientWithResponses
}

func (d *dimensionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dimension"
}

func (d *dimensionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = client
}

func (d *dimensionDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_dimension.DimensionDataSourceSchema(ctx)
}

func (d *dimensionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_dimension.DimensionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If required inputs are unknown (depend on resources not yet created),
	// set all computed attributes to unknown so consumers don't treat null as
	// a real value during planning.
	if state.Id.IsUnknown() || state.Type.IsUnknown() {
		state.Label = types.StringUnknown()
		state.Values = types.ListUnknown(datasource_dimension.ValuesValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Build query parameters
	params := &models.GetDimensionsParams{
		Type: models.DimensionsTypes(state.Type.ValueString()),
		Id:   state.Id.ValueString(),
	}

	apiResp, err := d.client.GetDimensionsWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error reading dimension", err.Error())
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Dimension not found",
			fmt.Sprintf("Dimension with type %q and id %q not found", state.Type.ValueString(), state.Id.ValueString()),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading dimension",
			fmt.Sprintf("Unexpected status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	dimension := apiResp.JSON200

	// Map scalar fields — preserve user's configured type when the API
	// returns a canonical alias (e.g. user sends "allocation", API returns
	// "attribution_group"). Without this, re-reads would cause perpetual drift.
	userType := state.Type.ValueString()
	state.Id = types.StringPointerValue(dimension.Id)
	state.Label = types.StringPointerValue(dimension.Label)
	if dimension.Type != nil {
		state.Type = types.StringValue(normalizeDimensionsType(string(*dimension.Type), userType))
	}

	// Map values list
	if dimension.Values != nil && len(*dimension.Values) > 0 {
		valVals := make([]datasource_dimension.ValuesValue, 0, len(*dimension.Values))
		for _, v := range *dimension.Values {
			valVal, d := datasource_dimension.NewValuesValue(
				datasource_dimension.ValuesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"cloud": types.StringPointerValue(v.Cloud),
					"value": types.StringPointerValue(v.Value),
				},
			)
			resp.Diagnostics.Append(d...)
			valVals = append(valVals, valVal)
		}
		valuesList, d := types.ListValueFrom(ctx, datasource_dimension.ValuesValue{}.Type(ctx), valVals)
		resp.Diagnostics.Append(d...)
		state.Values = valuesList
	} else {
		emptyList, d := types.ListValueFrom(ctx, datasource_dimension.ValuesValue{}.Type(ctx), []datasource_dimension.ValuesValue{})
		resp.Diagnostics.Append(d...)
		state.Values = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
