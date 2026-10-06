package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_allocation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/oapi-codegen/nullable"
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
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (ds *allocationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (ds *allocationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_allocation.AllocationDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
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

	readTimeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
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
		data.Description = types.StringUnknown()
		data.FolderId = types.StringUnknown()
		data.Type = types.StringUnknown()
		data.AllocationType = types.StringUnknown()
		data.AnomalyDetection = types.BoolUnknown()
		data.CreateTime = types.Int64Unknown()
		data.UpdateTime = types.Int64Unknown()
		data.UnallocatedCosts = types.StringUnknown()
		data.Rule = datasource_allocation.NewRuleValueUnknown()
		data.Rules = types.ListUnknown(datasource_allocation.RulesValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
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
	data.UnallocatedCosts = types.StringPointerValue(nullableToPointer(allocation.UnallocatedCosts))
	data.FolderId = types.StringPointerValue(allocation.FolderId)

	if allocation.AllocationType != nil {
		data.AllocationType = types.StringValue(string(*allocation.AllocationType))
	} else {
		data.AllocationType = types.StringNull()
	}

	// Map single Rule
	if rule := nullableToPointer(allocation.Rule); rule != nil {
		ruleMap := map[string]attr.Value{
			"formula": types.StringValue(rule.Formula),
		}
		componentsList, componentDiags := ds.mapComponentsToList(ctx, rule.Components)
		diags.Append(componentDiags...)
		if diags.HasError() {
			return diags
		}
		ruleMap["components"] = componentsList

		vpList, vpDiags := ds.mapValidityPeriodsToList(ctx, rule.ValidityPeriods)
		diags.Append(vpDiags...)
		if diags.HasError() {
			return diags
		}
		ruleMap["validity_periods"] = vpList

		var ruleDiags diag.Diagnostics
		data.Rule, ruleDiags = datasource_allocation.NewRuleValue(datasource_allocation.RuleValue{}.AttributeTypes(ctx), ruleMap)
		diags.Append(ruleDiags...)
		if diags.HasError() {
			return diags
		}
	} else {
		data.Rule = datasource_allocation.NewRuleValueNull()
	}

	// Map Rules list (for group allocations)
	if allocation.Rules != nil && len(*allocation.Rules) > 0 {
		rules := make([]attr.Value, 0, len(*allocation.Rules))
		for _, ruleNullable := range *allocation.Rules {
			rulePtr := nullableToPointer(ruleNullable)
			if rulePtr == nil {
				continue
			}
			rule := *rulePtr
			ruleMap := map[string]attr.Value{
				"action":      types.StringValue("select"), // Default action - not returned by API
				"description": types.StringPointerValue(rule.Description),
				"formula":     types.StringValue(""),
				"id":          types.StringPointerValue(rule.Id),
				"name":        types.StringPointerValue(rule.Name),
			}

			if (rule.Formula == nil || rule.Components == nil || !rule.ValidityPeriods.IsSpecified() || !rule.ValueExtraction.IsSpecified()) && rule.Id != nil && ds.client != nil {
				respHTTPFullAlloc, err := ds.client.GetAllocationWithResponse(ctx, *rule.Id)
				if err == nil && respHTTPFullAlloc.JSON200 != nil {
					fullAlloc := respHTTPFullAlloc.JSON200
					if rulePtr := nullableToPointer(fullAlloc.Rule); rulePtr != nil {
						if rule.Formula == nil {
							rule.Formula = &rulePtr.Formula
						}
						if rule.Components == nil && rulePtr.Components != nil {
							rule.Components = &rulePtr.Components
						}
						if !rule.ValidityPeriods.IsSpecified() && rulePtr.ValidityPeriods.IsSpecified() {
							rule.ValidityPeriods = rulePtr.ValidityPeriods
						}
					}
					var allocDetail struct {
						Rule struct {
							ValueExtraction nullable.Nullable[models.AllocationValueExtraction] `json:"valueExtraction"`
						} `json:"rule"`
					}
					if uerr := json.Unmarshal(respHTTPFullAlloc.Body, &allocDetail); uerr == nil {
						if !rule.ValueExtraction.IsSpecified() && allocDetail.Rule.ValueExtraction.IsSpecified() {
							rule.ValueExtraction = allocDetail.Rule.ValueExtraction
						}
					}
				}
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
				return diags
			}
			ruleMap["components"] = componentsList

			vpList, vpDiags := ds.mapValidityPeriodsToList(ctx, rule.ValidityPeriods)
			diags.Append(vpDiags...)
			if diags.HasError() {
				return diags
			}
			ruleMap["validity_periods"] = vpList

			veVal, veDiags := ds.mapValueExtraction(ctx, rule.ValueExtraction)
			diags.Append(veDiags...)
			if diags.HasError() {
				return diags
			}
			ruleMap["value_extraction"] = veVal

			var ruleDiags diag.Diagnostics
			ruleVal, ruleDiags := datasource_allocation.NewRulesValue(datasource_allocation.RulesValue{}.AttributeTypes(ctx), ruleMap)
			diags.Append(ruleDiags...)
			if diags.HasError() {
				return diags
			}
			rules = append(rules, ruleVal)
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
			"case_insensitive": types.BoolPointerValue(component.CaseInsensitive),
			"include_null":     types.BoolPointerValue(component.IncludeNull),
			"inverse":          types.BoolPointerValue(component.Inverse),
			"key":              types.StringValue(component.Key),
			"mode":             types.StringValue(string(component.Mode)),
			"type":             types.StringValue(string(component.Type)),
			"values":           valuesList,
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

// mapValidityPeriodsToList converts API rule periods to a Terraform List.
func (ds *allocationDataSource) mapValidityPeriodsToList(ctx context.Context, nullablePeriods nullable.Nullable[[]models.AllocationRulePeriod]) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	periods := nullableToPointer(nullablePeriods)
	if periods == nil || len(*periods) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_allocation.ValidityPeriodsValue{}.Type(ctx), []datasource_allocation.ValidityPeriodsValue{})
		diags.Append(d...)
		return emptyList, diags
	}

	vals := make([]attr.Value, len(*periods))
	for i, p := range *periods {
		m := map[string]attr.Value{
			"start_date": types.StringPointerValue(nullableToPointer(p.StartDate)),
			"end_date":   types.StringPointerValue(nullableToPointer(p.EndDate)),
		}
		vpVal, d := datasource_allocation.NewValidityPeriodsValue(datasource_allocation.ValidityPeriodsValue{}.AttributeTypes(ctx), m)
		diags.Append(d...)
		if diags.HasError() {
			emptyList, d2 := types.ListValueFrom(ctx, datasource_allocation.ValidityPeriodsValue{}.Type(ctx), []datasource_allocation.ValidityPeriodsValue{})
			diags.Append(d2...)
			return emptyList, diags
		}
		vals[i] = vpVal
	}
	res, d := types.ListValue(datasource_allocation.ValidityPeriodsValue{}.Type(ctx), vals)
	diags.Append(d...)
	return res, diags
}

// mapValueExtraction converts API value extraction to a Terraform ValueExtractionValue.
func (ds *allocationDataSource) mapValueExtraction(ctx context.Context, nullableVE nullable.Nullable[models.AllocationValueExtraction]) (datasource_allocation.ValueExtractionValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	ve := nullableToPointer(nullableVE)
	if ve == nil {
		return datasource_allocation.NewValueExtractionValueNull(), diags
	}

	sourceVals := make([]attr.Value, len(ve.Sources))
	for i, s := range ve.Sources {
		var provList basetypes.ListValue
		if provs := nullableToPointer(s.Providers); provs != nil {
			pVals := make([]attr.Value, len(*provs))
			for j, p := range *provs {
				pVals[j] = types.StringValue(p)
			}
			var d diag.Diagnostics
			provList, d = types.ListValue(types.StringType, pVals)
			diags.Append(d...)
			if diags.HasError() {
				return datasource_allocation.NewValueExtractionValueNull(), diags
			}
		} else {
			provList = types.ListNull(types.StringType)
		}

		mSource := map[string]attr.Value{
			"key":       types.StringValue(s.Key),
			"type":      types.StringValue(string(s.Type)),
			"providers": provList,
		}
		sVal, d := datasource_allocation.NewSourcesValue(datasource_allocation.SourcesValue{}.AttributeTypes(ctx), mSource)
		diags.Append(d...)
		if diags.HasError() {
			return datasource_allocation.NewValueExtractionValueNull(), diags
		}
		sourceVals[i] = sVal
	}

	sourcesList, d := types.ListValue(datasource_allocation.SourcesValue{}.Type(ctx), sourceVals)
	diags.Append(d...)
	if diags.HasError() {
		return datasource_allocation.NewValueExtractionValueNull(), diags
	}

	onMissingVal := types.StringValue("useFallback")
	if ve.OnMissing != nil {
		onMissingVal = types.StringValue(string(*ve.OnMissing))
	}

	fallbackVal := types.StringPointerValue(nullableToPointer(ve.Fallback))

	mVE := map[string]attr.Value{
		"fallback":   fallbackVal,
		"on_missing": onMissingVal,
		"sources":    sourcesList,
	}

	veVal, d := datasource_allocation.NewValueExtractionValue(datasource_allocation.ValueExtractionValue{}.AttributeTypes(ctx), mVE)
	diags.Append(d...)
	return veVal, diags
}
