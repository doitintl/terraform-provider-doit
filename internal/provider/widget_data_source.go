package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_widget"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*widgetDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*widgetDataSource)(nil)

func NewWidgetDataSource() datasource.DataSource {
	return &widgetDataSource{}
}

type widgetDataSource struct {
	client *models.ClientWithResponses
}

type widgetDataSourceModel struct {
	datasource_widget.WidgetModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *widgetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_widget"
}

func (d *widgetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *widgetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_widget.WidgetDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *widgetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data widgetDataSourceModel
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

	if data.WidgetId.IsUnknown() {
		data.Alias = types.StringUnknown()
		data.GenerateTime = types.StringUnknown()
		data.Id = types.StringUnknown()
		data.Kind = types.StringUnknown()
		data.Result = datasource_widget.NewResultValueUnknown()
		data.Type = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	widgetID := data.WidgetId.ValueString()
	apiResp, err := d.client.GetWidgetWithResponse(ctx, widgetID, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Widget",
			fmt.Sprintf("Unable to read widget: %v", err),
		)
		return
	}
	if apiResp.StatusCode() == 404 {
		if problem := apiResp.ApplicationproblemJSON404; problem != nil {
			switch problem.Code {
			case "analytics.widget_data_not_available":
				resp.Diagnostics.AddError(
					"Widget Data Not Available",
					fmt.Sprintf("Widget %s data has not been computed for this customer yet: %s", widgetID, problem.Detail),
				)
				return
			case "analytics.widget_no_billing_data":
				resp.Diagnostics.AddError(
					"No Billing Data Available",
					fmt.Sprintf("No billing data is available to generate widget %s: %s", widgetID, problem.Detail),
				)
				return
			case "not_found":
				resp.Diagnostics.AddError(
					"Widget Not Found",
					fmt.Sprintf("Widget %s not found: %s", widgetID, problem.Detail),
				)
				return
			case "":
				// No specific problem code provided, fall through to generic 404 below
			default:
				detail := problem.Detail
				if detail == "" {
					detail = string(apiResp.Body)
				}
				resp.Diagnostics.AddError(
					"Widget Unavailable",
					fmt.Sprintf("Widget %s unavailable (%s): %s", widgetID, problem.Code, detail),
				)
				return
			}
		}

		resp.Diagnostics.AddError(
			"Widget Not Found",
			fmt.Sprintf("Widget %s not found: %s", widgetID, string(apiResp.Body)),
		)
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Widget",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapWidgetToModel(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapWidgetToModel(ctx context.Context, widget *models.Widget, data *widgetDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringValue(widget.Id)
	data.Alias = types.StringValue(widget.Alias)
	data.Type = types.StringValue(string(widget.Type))
	data.Kind = types.StringValue(string(widget.Kind))
	data.GenerateTime = types.StringValue(widget.GenerateTime.UTC().Format(time.RFC3339Nano))

	metric := widget.Result.MonetaryMetric

	valVal, d := datasource_widget.NewValueValue(
		datasource_widget.ValueValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   types.StringValue(metric.Value.Amount),
			"currency": types.StringValue(metric.Value.Currency),
		},
	)
	diags.Append(d...)

	periodVal, d := datasource_widget.NewPeriodValue(
		datasource_widget.PeriodValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"start_time": types.StringValue(metric.Period.StartTime.UTC().Format(time.RFC3339)),
			"end_time":   types.StringValue(metric.Period.EndTime.UTC().Format(time.RFC3339)),
		},
	)
	diags.Append(d...)

	var growthAmtVal datasource_widget.MonthToMonthGrowthAmountValue
	if amt := nullableToPointer(metric.MonthToMonthGrowthAmount); amt != nil {
		var growthDiags diag.Diagnostics
		growthAmtVal, growthDiags = datasource_widget.NewMonthToMonthGrowthAmountValue(
			datasource_widget.MonthToMonthGrowthAmountValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"amount":   types.StringValue(amt.Amount),
				"currency": types.StringValue(amt.Currency),
			},
		)
		diags.Append(growthDiags...)
	} else {
		growthAmtVal = datasource_widget.NewMonthToMonthGrowthAmountValueNull()
	}

	var pctVal types.Float64
	if pct := nullableToPointer(metric.MonthToMonthGrowthPercentage); pct != nil {
		pctVal = types.Float64Value(*pct)
	} else {
		pctVal = types.Float64Null()
	}

	monetaryMetricVal, d := datasource_widget.NewMonetaryMetricValue(
		datasource_widget.MonetaryMetricValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"month_to_month_growth_amount":     growthAmtVal,
			"month_to_month_growth_percentage": pctVal,
			"period":                           periodVal,
			"value":                            valVal,
		},
	)
	diags.Append(d...)

	resultVal, d := datasource_widget.NewResultValue(
		datasource_widget.ResultValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"monetary_metric": monetaryMetricVal,
		},
	)
	diags.Append(d...)

	data.Result = resultVal

	return diags
}
