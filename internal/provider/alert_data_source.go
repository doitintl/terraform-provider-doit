package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_alert"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*alertDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*alertDataSource)(nil)

func NewAlertDataSource() datasource.DataSource {
	return &alertDataSource{}
}

type alertDataSource struct {
	client *models.ClientWithResponses
}

func (ds *alertDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert"
}

func (ds *alertDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *alertDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_alert.AlertDataSourceSchema(ctx)
}

func (ds *alertDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_alert.AlertModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.Id.ValueString()
	alertResp, err := ds.client.GetAlertWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading alert", err.Error())
		return
	}
	if alertResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Alert not found", fmt.Sprintf("Alert with ID %s not found", id))
		return
	}
	if alertResp.StatusCode() != 200 || alertResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading alert",
			fmt.Sprintf("Unexpected status: %d, body: %s", alertResp.StatusCode(), string(alertResp.Body)),
		)
		return
	}

	alert := alertResp.JSON200
	resp.Diagnostics.Append(ds.mapAlertToModel(ctx, alert, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (ds *alertDataSource) mapAlertToModel(ctx context.Context, alert *models.Alert, state *datasource_alert.AlertModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(alert.Id)
	state.Name = types.StringValue(alert.Name)
	state.CreateTime = types.Int64PointerValue(alert.CreateTime)
	state.UpdateTime = types.Int64PointerValue(alert.UpdateTime)
	state.LastAlerted = types.Int64PointerValue(alert.LastAlerted)

	// Map recipients
	if alert.Recipients != nil {
		recipientsList, d := types.ListValueFrom(ctx, types.StringType, *alert.Recipients)
		diags.Append(d...)
		state.Recipients = recipientsList
	} else {
		state.Recipients = types.ListNull(types.StringType)
	}

	// Map config
	if alert.Config != nil {
		configVal, d := ds.mapConfigToModel(ctx, alert.Config)
		diags.Append(d...)
		state.Config = configVal
	} else {
		state.Config = datasource_alert.NewConfigValueNull()
	}

	return diags
}

func (ds *alertDataSource) mapConfigToModel(ctx context.Context, config *models.AlertConfig) (datasource_alert.ConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Map attributions
	var attributions types.List
	if config.Attributions != nil {
		attrList, d := types.ListValueFrom(ctx, types.StringType, *config.Attributions)
		diags.Append(d...)
		attributions = attrList
	} else {
		attributions = types.ListNull(types.StringType)
	}

	// Map condition
	var condition types.String
	if config.Condition != nil {
		condition = types.StringValue(*config.Condition)
	} else {
		condition = types.StringNull()
	}

	// Map currency
	var currency types.String
	if config.Currency != nil {
		currency = types.StringValue(string(*config.Currency))
	} else {
		currency = types.StringNull()
	}

	// Map data_source
	var dataSource types.String
	if config.DataSource != nil {
		dataSource = types.StringValue(*config.DataSource)
	} else {
		dataSource = types.StringNull()
	}

	// Map evaluate_for_each
	var evaluateForEach types.String
	if config.EvaluateForEach != nil {
		evaluateForEach = types.StringValue(*config.EvaluateForEach)
	} else {
		evaluateForEach = types.StringNull()
	}

	// Map operator
	var operator types.String
	if config.Operator != nil {
		operator = types.StringValue(*config.Operator)
	} else {
		operator = types.StringNull()
	}

	// Map time_interval
	timeInterval := types.StringValue(string(config.TimeInterval))

	// Map value
	value := types.Float64Value(config.Value)

	// Map metric
	metricVal, d := ds.mapMetricToModel(ctx, &config.Metric)
	diags.Append(d...)

	// Map scopes
	var scopes types.List
	if config.Scopes != nil && len(*config.Scopes) > 0 {
		scopeVals := make([]datasource_alert.ScopesValue, 0, len(*config.Scopes))
		for _, s := range *config.Scopes {
			scopeVal, scopeDiags := ds.mapScopeToModel(ctx, &s)
			diags.Append(scopeDiags...)
			scopeVals = append(scopeVals, scopeVal)
		}
		scopesList, scopesListDiags := types.ListValueFrom(ctx, datasource_alert.ScopesValue{}.Type(ctx), scopeVals)
		diags.Append(scopesListDiags...)
		scopes = scopesList
	} else {
		scopes = types.ListNull(datasource_alert.ScopesValue{}.Type(ctx))
	}

	configVal, d := datasource_alert.NewConfigValue(
		datasource_alert.ConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"attributions":      attributions,
			"condition":         condition,
			"currency":          currency,
			"data_source":       dataSource,
			"evaluate_for_each": evaluateForEach,
			"metric":            metricVal,
			"operator":          operator,
			"scopes":            scopes,
			"time_interval":     timeInterval,
			"value":             value,
		},
	)
	diags.Append(d...)

	return configVal, diags
}

func (ds *alertDataSource) mapMetricToModel(ctx context.Context, metric *models.MetricConfig) (datasource_alert.MetricValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	metricVal, d := datasource_alert.NewMetricValue(
		datasource_alert.MetricValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"type":  types.StringValue(metric.Type),
			"value": types.StringValue(metric.Value),
		},
	)
	diags.Append(d...)

	return metricVal, diags
}

func (ds *alertDataSource) mapScopeToModel(ctx context.Context, scope *models.ExternalConfigFilter) (datasource_alert.ScopesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Map values
	var values types.List
	if scope.Values != nil {
		valList, d := types.ListValueFrom(ctx, types.StringType, *scope.Values)
		diags.Append(d...)
		values = valList
	} else {
		values = types.ListNull(types.StringType)
	}

	// Map mode - Mode is not a pointer, it's a string type
	mode := types.StringValue(string(scope.Mode))

	// Map inverse
	var inverse types.Bool
	if scope.Inverse != nil {
		inverse = types.BoolValue(*scope.Inverse)
	} else {
		inverse = types.BoolNull()
	}

	scopeVal, d := datasource_alert.NewScopesValue(
		datasource_alert.ScopesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":      types.StringValue(scope.Id),
			"type":    types.StringValue(string(scope.Type)),
			"mode":    mode,
			"inverse": inverse,
			"values":  values,
		},
	)
	diags.Append(d...)

	return scopeVal, diags
}
