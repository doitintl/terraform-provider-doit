package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_budget_suggestions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*budgetSuggestionsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*budgetSuggestionsDataSource)(nil)

func NewBudgetSuggestionsDataSource() datasource.DataSource {
	return &budgetSuggestionsDataSource{}
}

type budgetSuggestionsDataSource struct {
	client *models.ClientWithResponses
}

type budgetSuggestionsDataSourceModel struct {
	datasource_budget_suggestions.BudgetSuggestionsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *budgetSuggestionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budget_suggestions"
}

func (d *budgetSuggestionsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_budget_suggestions.BudgetSuggestionsDataSourceSchema(ctx)

	s.MarkdownDescription = "List the pending AI-generated budget suggestions for the account."
	s.Description = "List the pending AI-generated budget suggestions for the account."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *budgetSuggestionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *budgetSuggestionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data budgetSuggestionsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	apiResp, err := d.client.ListBudgetSuggestionsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Budget Suggestions",
			fmt.Sprintf("Unable to read budget suggestions: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Budget Suggestions",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	var items []models.BudgetSuggestion
	if apiResp.JSON200.Items != nil {
		items = *apiResp.JSON200.Items
	}

	itemsList, itemsDiags := mapBudgetSuggestionsItems(ctx, items)
	resp.Diagnostics.Append(itemsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	if apiResp.JSON200.RowCount != nil {
		data.RowCount = types.Int64Value(*apiResp.JSON200.RowCount)
	} else {
		data.RowCount = types.Int64Value(int64(len(items)))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
