package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_commitment_policies"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cCommitmentPoliciesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cCommitmentPoliciesDataSource)(nil)

func NewPs4cCommitmentPoliciesDataSource() datasource.DataSource {
	return &ps4cCommitmentPoliciesDataSource{}
}

type ps4cCommitmentPoliciesDataSource struct {
	client *models.ClientWithResponses
}

type ps4cCommitmentPoliciesDataSourceModel struct {
	datasource_ps4c_commitment_policies.Ps4cCommitmentPoliciesModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cCommitmentPoliciesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_commitment_policies"
}

func (d *ps4cCommitmentPoliciesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cCommitmentPoliciesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_commitment_policies.Ps4cCommitmentPoliciesDataSourceSchema(ctx)

	s.MarkdownDescription = "List all PerfectScale for Commitments (PS4C) commitment policies available to the tenant."
	s.Description = "List all PerfectScale for Commitments (PS4C) commitment policies available to the tenant."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cCommitmentPoliciesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cCommitmentPoliciesDataSourceModel

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

	if data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Items = types.ListUnknown(datasource_ps4c_commitment_policies.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	params := &models.ListCommitmentPoliciesParams{}
	userControlsPagination := !data.MaxResults.IsNull()

	var allItems []models.CommitmentPolicy

	if userControlsPagination {
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListCommitmentPoliciesWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading PS4C Commitment Policies", fmt.Sprintf("Unable to read PS4C commitment policies: %v", err))
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error Reading PS4C Commitment Policies", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}

		result := apiResp.JSON200
		allItems = result.Items

		data.PageToken = types.StringPointerValue(nullableToPointer(result.PageToken))
		if result.RowCount > 0 {
			data.RowCount = types.Int64Value(result.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allItems)))
		}
	} else {
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		var lastRowCount int64
		for {
			apiResp, err := d.client.ListCommitmentPoliciesWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading PS4C Commitment Policies", fmt.Sprintf("Unable to read PS4C commitment policies: %v", err))
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError("Error Reading PS4C Commitment Policies", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
				return
			}

			result := apiResp.JSON200
			allItems = append(allItems, result.Items...)
			lastRowCount = result.RowCount

			pageToken := nullableToPointer(result.PageToken)
			if pageToken == nil || *pageToken == "" {
				break
			}
			params.PageToken = pageToken
		}

		if lastRowCount > 0 {
			data.RowCount = types.Int64Value(lastRowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allItems)))
		}
		data.PageToken = types.StringNull()
	}

	itemsList, itemsDiags := mapCommitmentPoliciesToItemsList(ctx, allItems)
	resp.Diagnostics.Append(itemsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
