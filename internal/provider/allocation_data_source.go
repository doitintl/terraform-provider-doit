package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_allocation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*allocationDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*allocationDataSource)(nil)

func NewAllocationDataSource() datasource.DataSource {
	return &allocationDataSource{}
}

type (
	allocationDataSource struct {
		client *models.ClientWithResponses
	}
	allocationDataSourceModel struct {
		datasource_allocation.AllocationModel
	}
)

func (ds *allocationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (ds *allocationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_allocation.AllocationDataSourceSchema(ctx)
}

func (ds *allocationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	ds.client = client
}

func (ds *allocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data allocationDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API to get allocation
	allocationResp, err := ds.client.GetAllocationWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Allocation",
			"Could not read allocation ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if allocationResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Allocation",
			fmt.Sprintf("Could not read allocation ID %s, status: %d, body: %s", data.Id.ValueString(), allocationResp.StatusCode(), string(allocationResp.Body)),
		)
		return
	}

	if allocationResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Allocation",
			"Received empty response body for allocation ID "+data.Id.ValueString(),
		)
		return
	}

	allocation := allocationResp.JSON200

	// Map API response to model
	resp.Diagnostics.Append(ds.mapAllocationToModel(ctx, allocation, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAllocationToModel maps the API Allocation response to the data source model.
func (ds *allocationDataSource) mapAllocationToModel(ctx context.Context, allocation *models.Allocation, data *allocationDataSourceModel) (diags diag.Diagnostics) {
	data.Id = types.StringPointerValue(allocation.Id)
	data.Name = types.StringPointerValue(allocation.Name)
	data.Description = types.StringPointerValue(allocation.Description)
	data.Type = types.StringPointerValue(allocation.Type)
	data.AnomalyDetection = types.BoolPointerValue(allocation.AnomalyDetection)
	data.CreateTime = types.Int64PointerValue(allocation.CreateTime)
	data.UpdateTime = types.Int64PointerValue(allocation.UpdateTime)
	data.UnallocatedCosts = types.StringPointerValue(allocation.UnallocatedCosts)

	if allocation.AllocationType != nil {
		data.AllocationType = types.StringValue(string(*allocation.AllocationType))
	} else {
		data.AllocationType = types.StringNull()
	}

	// Map single Rule
	if allocation.Rule != nil {
		ruleMap := map[string]attr.Value{
			"formula": types.StringValue(allocation.Rule.Formula),
		}
		componentsList, componentDiags := ds.mapComponentsToList(ctx, allocation.Rule.Components)
		diags.Append(componentDiags...)
		if diags.HasError() {
			return
		}
		ruleMap["components"] = componentsList

		var ruleDiags diag.Diagnostics
		data.Rule, ruleDiags = datasource_allocation.NewRuleValue(datasource_allocation.RuleValue{}.AttributeTypes(ctx), ruleMap)
		diags.Append(ruleDiags...)
		if diags.HasError() {
			return
		}
	} else {
		data.Rule = datasource_allocation.NewRuleValueNull()
	}

	// Map Rules list (for group allocations)
	if allocation.Rules != nil && len(*allocation.Rules) > 0 {
		rules := make([]attr.Value, len(*allocation.Rules))
		for i, rule := range *allocation.Rules {
			ruleMap := map[string]attr.Value{
				"action":      types.StringValue("select"), // Default action - not returned by API
				"description": types.StringPointerValue(rule.Description),
				"formula":     types.StringValue(""),
				"id":          types.StringPointerValue(rule.Id),
				"name":        types.StringPointerValue(rule.Name),
			}

			if rule.Formula != nil {
				ruleMap["formula"] = types.StringValue(*rule.Formula)
			}

			var ruleComponents []models.AllocationComponent
			if rule.Components != nil {
				ruleComponents = *rule.Components
			}
			componentsList, componentDiags := ds.mapComponentsToList(ctx, ruleComponents)
			diags.Append(componentDiags...)
			if diags.HasError() {
				return
			}
			ruleMap["components"] = componentsList

			var ruleDiags diag.Diagnostics
			rules[i], ruleDiags = datasource_allocation.NewRulesValue(datasource_allocation.RulesValue{}.AttributeTypes(ctx), ruleMap)
			diags.Append(ruleDiags...)
			if diags.HasError() {
				return
			}
		}
		var rulesListDiags diag.Diagnostics
		data.Rules, rulesListDiags = types.ListValueFrom(ctx, datasource_allocation.RulesValue{}.Type(ctx), rules)
		diags.Append(rulesListDiags...)
	} else {
		emptyRules, d := types.ListValueFrom(ctx, datasource_allocation.RulesValue{}.Type(ctx), []datasource_allocation.RulesValue{})
		diags.Append(d...)
		data.Rules = emptyRules
	}

	return diags
}

// mapComponentsToList converts API components to a Terraform List.
func (ds *allocationDataSource) mapComponentsToList(ctx context.Context, components []models.AllocationComponent) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(components) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_allocation.ComponentsValue{}.Type(ctx), []datasource_allocation.ComponentsValue{})
		diags.Append(d...)
		return emptyList, diags
	}

	componentValues := make([]attr.Value, len(components))
	for i, component := range components {
		valuesAttrList := make([]attr.Value, len(component.Values))
		for j, v := range component.Values {
			valuesAttrList[j] = types.StringValue(v)
		}
		valuesList, valsDiags := types.ListValue(types.StringType, valuesAttrList)
		diags.Append(valsDiags...)
		if diags.HasError() {
			emptyList, d := types.ListValueFrom(ctx, datasource_allocation.ComponentsValue{}.Type(ctx), []datasource_allocation.ComponentsValue{})
			diags.Append(d...)
			return emptyList, diags
		}

		componentMap := map[string]attr.Value{
			"include_null":      types.BoolPointerValue(component.IncludeNull),
			"inverse_selection": types.BoolPointerValue(component.InverseSelection),
			"key":               types.StringValue(component.Key),
			"mode":              types.StringValue(string(component.Mode)),
			"type":              types.StringValue(string(component.Type)),
			"values":            valuesList,
		}

		componentValue, compDiags := datasource_allocation.NewComponentsValue(datasource_allocation.ComponentsValue{}.AttributeTypes(ctx), componentMap)
		diags.Append(compDiags...)
		if diags.HasError() {
			emptyList, d := types.ListValueFrom(ctx, datasource_allocation.ComponentsValue{}.Type(ctx), []datasource_allocation.ComponentsValue{})
			diags.Append(d...)
			return emptyList, diags
		}
		componentValues[i] = componentValue
	}

	componentsList, listDiags := types.ListValueFrom(ctx, datasource_allocation.ComponentsValue{}.Type(ctx), componentValues)
	diags.Append(listDiags...)
	return componentsList, diags
}
