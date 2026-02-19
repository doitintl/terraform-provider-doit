package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_annotation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*annotationDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*annotationDataSource)(nil)

func NewAnnotationDataSource() datasource.DataSource {
	return &annotationDataSource{}
}

type (
	annotationDataSource struct {
		client *models.ClientWithResponses
	}
	annotationDataSourceModel struct {
		datasource_annotation.AnnotationModel
	}
)

func (d *annotationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation"
}

func (d *annotationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_annotation.AnnotationDataSourceSchema(ctx)
}

func (d *annotationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = client
}

func (d *annotationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data annotationDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API to get annotation
	annotationResp, err := d.client.GetAnnotationWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Annotation",
			"Could not read annotation ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if annotationResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Annotation",
			fmt.Sprintf("Could not read annotation ID %s, status: %d, body: %s", data.Id.ValueString(), annotationResp.StatusCode(), string(annotationResp.Body)),
		)
		return
	}

	if annotationResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Annotation",
			"Received empty response body for annotation ID "+data.Id.ValueString(),
		)
		return
	}

	annotation := annotationResp.JSON200

	// Map API response to model
	data.Id = types.StringValue(annotation.Id)
	data.Content = types.StringValue(annotation.Content)
	data.Timestamp = types.StringValue(annotation.Timestamp.UTC().Format(time.RFC3339))

	if annotation.CreateTime != nil {
		data.CreateTime = types.StringValue(annotation.CreateTime.UTC().Format(time.RFC3339))
	} else {
		data.CreateTime = types.StringNull()
	}

	if annotation.UpdateTime != nil {
		data.UpdateTime = types.StringValue(annotation.UpdateTime.UTC().Format(time.RFC3339))
	} else {
		data.UpdateTime = types.StringNull()
	}

	// Map reports list
	if annotation.Reports != nil && len(*annotation.Reports) > 0 {
		reportsList, diags := types.ListValueFrom(ctx, types.StringType, *annotation.Reports)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Reports = reportsList
	} else {
		emptyList1, d := types.ListValueFrom(ctx, types.StringType, []string{})
		resp.Diagnostics.Append(d...)
		data.Reports = emptyList1
	}

	// Map labels list
	if annotation.Labels != nil && len(*annotation.Labels) > 0 {
		labelValues := make([]attr.Value, len(*annotation.Labels))
		for i, label := range *annotation.Labels {
			labelAttrs := map[string]attr.Value{
				"id":   types.StringValue(label.Id),
				"name": types.StringValue(label.Name),
			}
			labelValue, diags := datasource_annotation.NewLabelsValue(
				datasource_annotation.LabelsValue{}.AttributeTypes(ctx),
				labelAttrs,
			)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			labelValues[i] = labelValue
		}
		labelsList, diags := types.ListValueFrom(ctx, datasource_annotation.LabelsValue{}.Type(ctx), labelValues)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Labels = labelsList
	} else {
		emptyLabels, d := types.ListValueFrom(ctx, datasource_annotation.LabelsValue{}.Type(ctx), []datasource_annotation.LabelsValue{})
		resp.Diagnostics.Append(d...)
		data.Labels = emptyLabels
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
