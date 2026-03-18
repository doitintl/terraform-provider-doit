package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_label_assignments"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*labelAssignmentsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*labelAssignmentsDataSource)(nil)

func NewLabelAssignmentsDataSource() datasource.DataSource {
	return &labelAssignmentsDataSource{}
}

type labelAssignmentsDataSource struct {
	client *models.ClientWithResponses
}

func (d *labelAssignmentsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label_assignments"
}

func (d *labelAssignmentsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_label_assignments.LabelAssignmentsDataSourceSchema(ctx)
	resp.Schema.Description = "Retrieves the list of objects (reports, budgets, alerts, etc.) assigned to a specific label."
	resp.Schema.MarkdownDescription = "Retrieves the list of objects (reports, budgets, alerts, etc.) assigned to a specific label."
}

func (d *labelAssignmentsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *labelAssignmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_label_assignments.LabelAssignmentsModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is unknown (depends on a resource not yet created), return early
	// but set computed list attributes to Unknown and preserve state during planning.
	if data.Id.IsUnknown() {
		elemType := datasource_label_assignments.AssignmentsType{
			ObjectType: types.ObjectType{
				AttrTypes: datasource_label_assignments.AssignmentsValue{}.AttributeTypes(ctx),
			},
		}
		data.Assignments = types.ListUnknown(elemType)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	labelID := data.Id.ValueString()
	apiResp, err := d.client.GetLabelAssignmentsWithResponse(ctx, labelID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Label Assignments",
			fmt.Sprintf("Could not read label assignments for label %q: %s", labelID, err),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Label Assignments",
			fmt.Sprintf("API returned status %d for label %q: %s", apiResp.StatusCode(), labelID, string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Label Assignments",
			fmt.Sprintf("API returned empty body for label %q", labelID),
		)
		return
	}

	// Map assignments from API response to Terraform state
	elemType := datasource_label_assignments.AssignmentsType{
		ObjectType: types.ObjectType{
			AttrTypes: datasource_label_assignments.AssignmentsValue{}.AttributeTypes(ctx),
		},
	}

	if apiResp.JSON200.Assignments != nil && len(*apiResp.JSON200.Assignments) > 0 {
		assignmentVals := make([]datasource_label_assignments.AssignmentsValue, len(*apiResp.JSON200.Assignments))
		for i, a := range *apiResp.JSON200.Assignments {
			val, diags := datasource_label_assignments.NewAssignmentsValue(
				datasource_label_assignments.AssignmentsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"object_id":   types.StringValue(a.ObjectId),
					"object_type": types.StringValue(string(a.ObjectType)),
				},
			)
			resp.Diagnostics.Append(diags...)
			assignmentVals[i] = val
		}
		if resp.Diagnostics.HasError() {
			return
		}
		var listDiags diag.Diagnostics
		data.Assignments, listDiags = types.ListValueFrom(ctx, elemType, assignmentVals)
		resp.Diagnostics.Append(listDiags...)
	} else {
		var emptyDiags diag.Diagnostics
		data.Assignments, emptyDiags = types.ListValueFrom(ctx, elemType, []datasource_label_assignments.AssignmentsValue{})
		resp.Diagnostics.Append(emptyDiags...)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
