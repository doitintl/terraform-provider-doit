package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_anomaly_explanation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*anomalyExplanationDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*anomalyExplanationDataSource)(nil)

func NewAnomalyExplanationDataSource() datasource.DataSource {
	return &anomalyExplanationDataSource{}
}

type anomalyExplanationDataSource struct {
	client *models.ClientWithResponses
}

type anomalyExplanationDataSourceModel struct {
	datasource_anomaly_explanation.AnomalyExplanationModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (ds *anomalyExplanationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anomaly_explanation"
}

func (ds *anomalyExplanationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *anomalyExplanationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_anomaly_explanation.AnomalyExplanationDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (ds *anomalyExplanationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data anomalyExplanationDataSourceModel
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
		data.Facts = datasource_anomaly_explanation.NewFactsValueUnknown()
		data.Evidence = types.ListUnknown(datasource_anomaly_explanation.EvidenceValue{}.Type(ctx))
		data.Explanation = datasource_anomaly_explanation.NewExplanationValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	id := data.Id.ValueString()
	explanationResp, err := ds.client.GetAnomalyExplanationWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading anomaly explanation", err.Error())
		return
	}
	if explanationResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Anomaly explanation not found", fmt.Sprintf("Anomaly with ID %s not found", id))
		return
	}
	if explanationResp.StatusCode() != 200 || explanationResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading anomaly explanation",
			fmt.Sprintf("Unexpected status: %d, body: %s", explanationResp.StatusCode(), string(explanationResp.Body)),
		)
		return
	}

	body := explanationResp.JSON200

	// Map facts (deterministic, not AI-generated). The top3skus list inside facts
	// mirrors the singular anomaly data source's top3skus mapping.
	facts := body.Facts

	var top3skusList types.List
	if len(facts.Top3SKUs) > 0 {
		top3Vals := make([]datasource_anomaly_explanation.Top3skusValue, 0, len(facts.Top3SKUs))
		for _, sku := range facts.Top3SKUs {
			skuVal, d := datasource_anomaly_explanation.NewTop3skusValue(
				datasource_anomaly_explanation.Top3skusValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"cost": types.Float64PointerValue(sku.Cost),
					"name": types.StringPointerValue(sku.Name),
				},
			)
			resp.Diagnostics.Append(d...)
			top3Vals = append(top3Vals, skuVal)
		}
		list, d := types.ListValueFrom(ctx, datasource_anomaly_explanation.Top3skusValue{}.Type(ctx), top3Vals)
		resp.Diagnostics.Append(d...)
		top3skusList = list
	} else {
		emptyList, d := types.ListValueFrom(ctx, datasource_anomaly_explanation.Top3skusValue{}.Type(ctx), []datasource_anomaly_explanation.Top3skusValue{})
		resp.Diagnostics.Append(d...)
		top3skusList = emptyList
	}

	factsVal, d := datasource_anomaly_explanation.NewFactsValue(
		datasource_anomaly_explanation.FactsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"actual_cost":       types.Float64PointerValue(nullableToPointer(facts.ActualCost)),
			"cost_of_anomaly":   types.Float64Value(facts.CostOfAnomaly),
			"expected_max_cost": types.Float64PointerValue(nullableToPointer(facts.ExpectedMaxCost)),
			"platform":          types.StringValue(facts.Platform),
			"scope":             types.StringValue(facts.Scope),
			"service_name":      types.StringValue(facts.ServiceName),
			"severity_level":    types.StringValue(facts.SeverityLevel),
			"top3skus":          top3skusList,
		},
	)
	resp.Diagnostics.Append(d...)
	data.Facts = factsVal

	// Map explanation (AI-generated). text and generated_by are non-deterministic;
	// ai_generated is always true per the API contract.
	expl := body.Explanation
	explVal, d := datasource_anomaly_explanation.NewExplanationValue(
		datasource_anomaly_explanation.ExplanationValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ai_generated": types.BoolValue(expl.AiGenerated),
			"generated_by": types.StringValue(expl.GeneratedBy),
			"text":         types.StringValue(expl.Text),
		},
	)
	resp.Diagnostics.Append(d...)
	data.Explanation = explVal

	// Map evidence (list of references the explanation was grounded in).
	data.Evidence = mapEvidenceForAnomalyExplanation(ctx, body.Evidence, &resp.Diagnostics)

	data.Id = types.StringValue(id) // ID is not in the response, use the requested ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapEvidenceForAnomalyExplanation maps the API evidence slice to a Terraform list
// for the anomaly explanation data source. Always returns a non-null list (empty
// when the API omits or returns no evidence) so consumers can iterate safely.
func mapEvidenceForAnomalyExplanation(ctx context.Context, items []models.GetAnomalyExplanation200ResponseEvidenceItem, diagnostics *diag.Diagnostics) types.List {
	if len(items) == 0 {
		empty, d := types.ListValueFrom(ctx, datasource_anomaly_explanation.EvidenceValue{}.Type(ctx), []datasource_anomaly_explanation.EvidenceValue{})
		diagnostics.Append(d...)
		return empty
	}

	vals := make([]datasource_anomaly_explanation.EvidenceValue, 0, len(items))
	for _, item := range items {
		v, diags := datasource_anomaly_explanation.NewEvidenceValue(
			datasource_anomaly_explanation.EvidenceValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":   types.StringValue(item.Id),
				"type": types.StringValue(item.Type),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, v)
	}

	list, diags := types.ListValueFrom(ctx, datasource_anomaly_explanation.EvidenceValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}
