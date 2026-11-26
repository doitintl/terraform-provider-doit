package provider

import (
	"context"
	"fmt"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloudconnect_gcp_requirements"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ datasource.DataSource = (*cloudconnectGcpRequirementsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudconnectGcpRequirementsDataSource)(nil)

// NewCloudconnectGcpRequirementsDataSource creates a new GCP Cloud Connect requirements data source instance.
func NewCloudconnectGcpRequirementsDataSource() datasource.DataSource {
	return &cloudconnectGcpRequirementsDataSource{}
}

type cloudconnectGcpRequirementsDataSource struct {
	client *modelsv2.ClientWithResponses
}

// cloudconnectGcpRequirementsDataSourceModel adds the request inputs that the
// generator does not produce from the POST request body.
type cloudconnectGcpRequirementsDataSourceModel struct {
	ds.CloudconnectGcpRequirementsModel
	FeatureIds     types.Set      `tfsdk:"feature_ids"`
	FeatureInputs  types.Object   `tfsdk:"feature_inputs"`
	IdentityInputs types.Object   `tfsdk:"identity_inputs"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
}

type gcpRequirementsFeatureInputsModel struct {
	BigqueryFinops types.Object `tfsdk:"bigquery_finops"`
}

type gcpRequirementsBigqueryFinopsModel struct {
	Location types.String `tfsdk:"location"`
}

type gcpRequirementsIdentityInputsModel struct {
	ProjectId        types.String `tfsdk:"project_id"`
	ServiceAccountId types.String `tfsdk:"service_account_id"`
	RoleId           types.String `tfsdk:"role_id"`
}

func (d *cloudconnectGcpRequirementsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudconnect_gcp_requirements"
}

func (d *cloudconnectGcpRequirementsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	v2Client, err := newV2Client(client)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			"Could not create the v2 API client: "+err.Error()+". Please report this issue to the provider developers.",
		)
		return
	}

	d.client = v2Client
}

func (d *cloudconnectGcpRequirementsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Start from the generated schema (the response attributes) and add the
	// request inputs the generator cannot produce from the POST request body.
	// The response echoes the scope inputs, so they replace the generated
	// Computed attributes of the same names.
	s := ds.CloudconnectGcpRequirementsDataSourceSchema(ctx)

	s.Attributes["scope_kind"] = schema.StringAttribute{
		Required:            true,
		Description:         "Google Cloud resource to connect: `gcp_organization`, `gcp_project` or `gcp_dataset`.",
		MarkdownDescription: "Google Cloud resource to connect: `gcp_organization`, `gcp_project` or `gcp_dataset`.",
		Validators: []validator.String{
			stringvalidator.OneOf("gcp_organization", "gcp_project", "gcp_dataset"),
		},
	}
	s.Attributes["organization_id"] = schema.StringAttribute{
		Optional:            true,
		Description:         "Numeric Google Cloud organization ID, when `scope_kind` is `gcp_organization`.",
		MarkdownDescription: "Numeric Google Cloud organization ID, when `scope_kind` is `gcp_organization`.",
	}
	s.Attributes["project_id"] = schema.StringAttribute{
		Optional:            true,
		Description:         "Google Cloud project ID, when `scope_kind` is `gcp_project` or `gcp_dataset`.",
		MarkdownDescription: "Google Cloud project ID, when `scope_kind` is `gcp_project` or `gcp_dataset`.",
	}
	s.Attributes["dataset_id"] = schema.StringAttribute{
		Optional:            true,
		Description:         "BigQuery dataset ID, when `scope_kind` is `gcp_dataset`.",
		MarkdownDescription: "BigQuery dataset ID, when `scope_kind` is `gcp_dataset`.",
	}
	s.Attributes["feature_ids"] = schema.SetAttribute{
		ElementType:         types.StringType,
		Optional:            true,
		Description:         "Features to evaluate. Omit it to list every feature available for the scope.",
		MarkdownDescription: "Features to evaluate. Omit it to list every feature available for the scope.",
	}
	s.Attributes["feature_inputs"] = schema.SingleNestedAttribute{
		Optional:            true,
		Description:         "Inputs of features that need them.",
		MarkdownDescription: "Inputs of features that need them.",
		Attributes: map[string]schema.Attribute{
			"bigquery_finops": schema.SingleNestedAttribute{
				Optional:            true,
				Description:         "Inputs of the `bigquery-finops` feature.",
				MarkdownDescription: "Inputs of the `bigquery-finops` feature.",
				Attributes: map[string]schema.Attribute{
					"location": schema.StringAttribute{
						Optional:            true,
						Description:         "BigQuery location of the FinOps log-sink dataset. The API defaults it to `US`.",
						MarkdownDescription: "BigQuery location of the FinOps log-sink dataset. The API defaults it to `US`.",
					},
				},
			},
		},
	}
	s.Attributes["identity_inputs"] = schema.SingleNestedAttribute{
		Optional:            true,
		Description:         "Where to place the service account DoiT impersonates.",
		MarkdownDescription: "Where to place the service account DoiT impersonates.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional:            true,
				Description:         "Project that hosts the service account. Required when `scope_kind` is `gcp_organization`.",
				MarkdownDescription: "Project that hosts the service account. Required when `scope_kind` is `gcp_organization`.",
			},
			"service_account_id": schema.StringAttribute{
				Optional:            true,
				Description:         "Account ID of the service account (the part of the email before `@`). Required when `scope_kind` is `gcp_dataset`.",
				MarkdownDescription: "Account ID of the service account (the part of the email before `@`). Required when `scope_kind` is `gcp_dataset`.",
			},
			"role_id": schema.StringAttribute{
				Optional:            true,
				Description:         "ID of the custom IAM role to create.",
				MarkdownDescription: "ID of the custom IAM role to create.",
			},
		},
	}

	s.Description = "Resolves what to set up in Google Cloud before registering a `doit_cloudconnect_gcp_connection`: " +
		"the service account DoiT impersonates, the IAM permissions of each feature, setup artifacts and the current feature catalog version. " +
		"The read changes nothing."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	rejectNullDataSourceCollectionElements(s.Attributes)
	resp.Schema = s
}

func (d *cloudconnectGcpRequirementsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudconnectGcpRequirementsDataSourceModel

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

	// The results are not yet known; the inputs stay as configured.
	if !req.Config.Raw.IsFullyKnown() {
		data.CatalogVersion = types.StringUnknown()
		data.Preconditions = ds.NewPreconditionsValueUnknown()
		data.Identity = ds.NewIdentityValueUnknown()
		data.Features = types.ListUnknown(ds.FeaturesValue{}.Type(ctx))
		data.Artifacts = types.ListUnknown(ds.ArtifactsValue{}.Type(ctx))
		data.FollowUps = types.ListUnknown(ds.FollowUpsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	body, diags := data.toRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.ResolveGcpCloudConnectRequirementsWithResponse(ctx, &modelsv2.ResolveGcpCloudConnectRequirementsParams{}, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading GCP Cloud Connect Requirements",
			fmt.Sprintf("Unable to resolve GCP Cloud Connect requirements: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading GCP Cloud Connect Requirements",
			fmt.Sprintf("Unable to resolve GCP Cloud Connect requirements, status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapCloudconnectGcpRequirementsToModel(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// toRequest builds the requirements request from a fully known configuration.
func (data *cloudconnectGcpRequirementsDataSourceModel) toRequest(ctx context.Context) (modelsv2.ResolveGcpCloudConnectRequirementsJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	body := modelsv2.ResolveGcpCloudConnectRequirementsJSONRequestBody{
		ScopeKind:      modelsv2.GcpRequirementsRequestScopeKind(data.ScopeKind.ValueString()),
		OrganizationId: data.OrganizationId.ValueStringPointer(),
		ProjectId:      data.ProjectId.ValueStringPointer(),
		DatasetId:      data.DatasetId.ValueStringPointer(),
	}

	if !data.FeatureIds.IsNull() {
		var featureIDs []string
		diags.Append(data.FeatureIds.ElementsAs(ctx, &featureIDs, false)...)
		body.FeatureIds = &featureIDs
	}

	if !data.FeatureInputs.IsNull() {
		var featureInputs gcpRequirementsFeatureInputsModel
		diags.Append(data.FeatureInputs.As(ctx, &featureInputs, basetypes.ObjectAsOptions{})...)
		var inputs modelsv2.GcpFeatureInputs
		if !featureInputs.BigqueryFinops.IsNull() {
			var finops gcpRequirementsBigqueryFinopsModel
			diags.Append(featureInputs.BigqueryFinops.As(ctx, &finops, basetypes.ObjectAsOptions{})...)
			inputs.BigqueryFinops.Set(modelsv2.GcpBigQueryFinOpsInputs{
				Location: pointerToNullable(finops.Location.ValueStringPointer()),
			})
		}
		body.FeatureInputs = &inputs
	}

	if !data.IdentityInputs.IsNull() {
		var identityInputs gcpRequirementsIdentityInputsModel
		diags.Append(data.IdentityInputs.As(ctx, &identityInputs, basetypes.ObjectAsOptions{})...)
		body.IdentityInputs = &modelsv2.GcpRequirementsRequestIdentityInputs{
			ProjectId:        identityInputs.ProjectId.ValueStringPointer(),
			ServiceAccountId: identityInputs.ServiceAccountId.ValueStringPointer(),
			RoleId:           identityInputs.RoleId.ValueStringPointer(),
		}
	}

	return body, diags
}
