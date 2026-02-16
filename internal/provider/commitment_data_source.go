package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_commitment"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ datasource.DataSource = &commitmentDataSource{}

func NewCommitmentDataSource() datasource.DataSource {
	return &commitmentDataSource{}
}

type commitmentDataSource struct {
	client *models.ClientWithResponses
}

// commitmentDataSourceModel extends the generated model to rename 'provider' to 'cloud_provider'.
type commitmentDataSourceModel struct {
	CreateTime             types.Int64   `tfsdk:"create_time"`
	Currency               types.String  `tfsdk:"currency"`
	EndDate                types.String  `tfsdk:"end_date"`
	Id                     types.String  `tfsdk:"id"`
	Name                   types.String  `tfsdk:"name"`
	Periods                types.List    `tfsdk:"periods"`
	CloudProvider          types.String  `tfsdk:"cloud_provider"`
	StartDate              types.String  `tfsdk:"start_date"`
	TotalCommitmentValue   types.Float64 `tfsdk:"total_commitment_value"`
	TotalCurrentAttainment types.Float64 `tfsdk:"total_current_attainment"`
	UpdateTime             types.Int64   `tfsdk:"update_time"`
}

func (ds *commitmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_commitment"
}

func (ds *commitmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	ds.client = client
}

func (ds *commitmentDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	generated := datasource_commitment.CommitmentDataSourceSchema(ctx)
	// Rename 'provider' -> 'cloud_provider' (provider is a reserved Terraform attribute name)
	providerAttr := generated.Attributes["provider"]
	delete(generated.Attributes, "provider")
	generated.Attributes["cloud_provider"] = datasourceschema.StringAttribute{
		Computed:            providerAttr.(datasourceschema.StringAttribute).Computed,
		Description:         providerAttr.(datasourceschema.StringAttribute).Description,
		MarkdownDescription: providerAttr.(datasourceschema.StringAttribute).MarkdownDescription,
	}
	generated.Description = "Retrieves details of a specific commitment contract."
	generated.MarkdownDescription = generated.Description
	resp.Schema = generated
}

func (ds *commitmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state commitmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.Id.ValueString()
	apiResp, err := ds.client.GetCommitmentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading commitment", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Commitment not found",
			fmt.Sprintf("Commitment with ID %s not found", id))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API response",
			fmt.Sprintf("Status: %d, Body: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	commitment := apiResp.JSON200

	// Map simple fields
	if commitment.Name != nil {
		state.Name = types.StringValue(*commitment.Name)
	} else {
		state.Name = types.StringNull()
	}
	if commitment.Currency != nil {
		state.Currency = types.StringValue(*commitment.Currency)
	} else {
		state.Currency = types.StringNull()
	}
	if commitment.Provider != nil {
		state.CloudProvider = types.StringValue(string(*commitment.Provider))
	} else {
		state.CloudProvider = types.StringNull()
	}
	if commitment.CreateTime != nil {
		state.CreateTime = types.Int64Value(*commitment.CreateTime)
	} else {
		state.CreateTime = types.Int64Null()
	}
	if commitment.UpdateTime != nil {
		state.UpdateTime = types.Int64Value(*commitment.UpdateTime)
	} else {
		state.UpdateTime = types.Int64Null()
	}
	if commitment.TotalCommitmentValue != nil {
		state.TotalCommitmentValue = types.Float64Value(*commitment.TotalCommitmentValue)
	} else {
		state.TotalCommitmentValue = types.Float64Null()
	}
	if commitment.TotalCurrentAttainment != nil {
		state.TotalCurrentAttainment = types.Float64Value(*commitment.TotalCurrentAttainment)
	} else {
		state.TotalCurrentAttainment = types.Float64Null()
	}

	// Map date-time fields (time.Time -> string via RFC3339)
	if commitment.StartDate != nil {
		state.StartDate = types.StringValue(commitment.StartDate.Format("2006-01-02"))
	} else {
		state.StartDate = types.StringNull()
	}
	if commitment.EndDate != nil {
		state.EndDate = types.StringValue(commitment.EndDate.Format("2006-01-02"))
	} else {
		state.EndDate = types.StringNull()
	}

	// Map periods
	state.Periods = mapCommitmentPeriods(ctx, commitment.Periods)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// mapCommitmentPeriods converts API period models to Terraform periods list value.
func mapCommitmentPeriods(ctx context.Context, periods *[]models.CommitmentPeriod) types.List {
	periodsType := datasource_commitment.PeriodsType{
		ObjectType: types.ObjectType{
			AttrTypes: datasource_commitment.PeriodsValue{}.AttributeTypes(ctx),
		},
	}
	if periods == nil || len(*periods) == 0 {
		return types.ListValueMust(periodsType, []attr.Value{})
	}

	periodValues := make([]attr.Value, 0, len(*periods))
	for _, p := range *periods {
		var commitmentValue basetypes.Float64Value
		if p.CommitmentValue != nil {
			commitmentValue = types.Float64Value(*p.CommitmentValue)
		} else {
			commitmentValue = types.Float64Null()
		}

		var marketplaceLimitPercentage basetypes.Float64Value
		if p.MarketplaceLimitPercentage != nil {
			marketplaceLimitPercentage = types.Float64Value(*p.MarketplaceLimitPercentage)
		} else {
			marketplaceLimitPercentage = types.Float64Null()
		}

		var startDate basetypes.StringValue
		if p.StartDate != nil {
			startDate = types.StringValue(p.StartDate.Format("2006-01-02"))
		} else {
			startDate = types.StringNull()
		}

		var endDate basetypes.StringValue
		if p.EndDate != nil {
			endDate = types.StringValue(p.EndDate.Format("2006-01-02"))
		} else {
			endDate = types.StringNull()
		}

		pv := datasource_commitment.NewPeriodsValueMust(
			datasource_commitment.PeriodsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"commitment_value":             commitmentValue,
				"end_date":                     endDate,
				"marketplace_limit_percentage": marketplaceLimitPercentage,
				"start_date":                   startDate,
			},
		)
		periodValues = append(periodValues, pv)
	}

	return types.ListValueMust(periodsType, periodValues)
}
