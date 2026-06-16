package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloudconnect_aws_account"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*cloudconnectAwsAccountDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*cloudconnectAwsAccountDataSource)(nil)
)

func NewCloudconnectAwsAccountDataSource() datasource.DataSource {
	return &cloudconnectAwsAccountDataSource{}
}

type (
	cloudconnectAwsAccountDataSource struct {
		client *models.ClientWithResponses
	}
	cloudconnectAwsAccountDataSourceModel struct {
		datasource_cloudconnect_aws_account.CloudconnectAwsAccountModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (d *cloudconnectAwsAccountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudconnect_aws_account"
}

func (d *cloudconnectAwsAccountDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_cloudconnect_aws_account.CloudconnectAwsAccountDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieve a CloudConnect AWS account by its AWS account ID."
	s.Description = "Retrieve a CloudConnect AWS account by its AWS account ID."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *cloudconnectAwsAccountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudconnectAwsAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudconnectAwsAccountDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	if data.AccountId.IsUnknown() {
		data.RoleArn = types.StringUnknown()
		data.EnabledFeatures = types.ListUnknown(types.StringType)
		data.S3bucket = types.StringUnknown()
		data.S3bucketRegion = types.StringUnknown()
		data.SupportedFeatures = types.ListUnknown(datasource_cloudconnect_aws_account.SupportedFeaturesType{
			ObjectType: types.ObjectType{
				AttrTypes: datasource_cloudconnect_aws_account.SupportedFeaturesValue{}.AttributeTypes(ctx),
			},
		})
		data.TimeLinked = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	accountResp, err := d.client.GetAwsAccountWithResponse(ctx, data.AccountId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading CloudConnect AWS Account",
			"Could not read CloudConnect AWS account ID "+data.AccountId.ValueString()+": "+err.Error(),
		)
		return
	}

	if accountResp.StatusCode() != 200 || accountResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading CloudConnect AWS Account",
			fmt.Sprintf("Could not read CloudConnect AWS account ID %s, status: %d, body: %s",
				data.AccountId.ValueString(), accountResp.StatusCode(), string(accountResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapCloudConnectAwsAccountToDataSourceModel(ctx, accountResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapCloudConnectAwsAccountToDataSourceModel maps the API response to the data
// source model. This is separate from the resource mapping because the data
// source uses its own generated SupportedFeaturesValue type.
func mapCloudConnectAwsAccountToDataSourceModel(ctx context.Context, resp *models.AwsAccountResponse, data *cloudconnectAwsAccountDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.AccountId = types.StringPointerValue(resp.AccountID)
	data.RoleArn = types.StringPointerValue(resp.RoleArn)
	data.TimeLinked = types.StringPointerValue(resp.TimeLinked)
	data.S3bucket = types.StringPointerValue(resp.S3Bucket)
	data.S3bucketRegion = types.StringPointerValue(resp.S3BucketRegion)

	if resp.EnabledFeatures != nil {
		elems := make([]attr.Value, 0, len(*resp.EnabledFeatures))
		for _, f := range *resp.EnabledFeatures {
			elems = append(elems, types.StringValue(f))
		}
		data.EnabledFeatures, diags = types.ListValue(types.StringType, elems)
		if diags.HasError() {
			return diags
		}
	} else {
		data.EnabledFeatures = types.ListValueMust(types.StringType, []attr.Value{})
	}

	featElems := make([]datasource_cloudconnect_aws_account.SupportedFeaturesValue, 0)
	if resp.SupportedFeatures != nil {
		for _, f := range *resp.SupportedFeatures {
			featVal, featDiags := datasource_cloudconnect_aws_account.NewSupportedFeaturesValue(
				datasource_cloudconnect_aws_account.SupportedFeaturesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"name":                     types.StringPointerValue(f.Name),
					"has_required_permissions": types.BoolPointerValue(f.HasRequiredPermissions),
				},
			)
			diags.Append(featDiags...)
			if featDiags.HasError() {
				return diags
			}
			featElems = append(featElems, featVal)
		}
	}

	featList, listDiags := types.ListValueFrom(ctx, datasource_cloudconnect_aws_account.SupportedFeaturesType{
		ObjectType: types.ObjectType{
			AttrTypes: datasource_cloudconnect_aws_account.SupportedFeaturesValue{}.AttributeTypes(ctx),
		},
	}, featElems)
	diags.Append(listDiags...)
	if listDiags.HasError() {
		return diags
	}
	data.SupportedFeatures = featList

	return diags
}
