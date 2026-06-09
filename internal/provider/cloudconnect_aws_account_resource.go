package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_aws_account"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type (
	cloudconnectAwsAccountResource struct {
		client *models.ClientWithResponses
	}
	cloudconnectAwsAccountResourceModel struct {
		resource_cloudconnect_aws_account.CloudconnectAwsAccountModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*cloudconnectAwsAccountResource)(nil)
	_ resource.ResourceWithConfigure        = (*cloudconnectAwsAccountResource)(nil)
	_ resource.ResourceWithImportState      = (*cloudconnectAwsAccountResource)(nil)
	_ resource.ResourceWithConfigValidators = (*cloudconnectAwsAccountResource)(nil)
)

// NewCloudconnectAwsAccountResource creates a new cloud connect AWS account resource instance.
func NewCloudconnectAwsAccountResource() resource.Resource {
	return &cloudconnectAwsAccountResource{}
}

// Configure adds the provider configured client to the resource.
func (r *cloudconnectAwsAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *cloudconnectAwsAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudconnect_aws_account"
}

func (r *cloudconnectAwsAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("account_id"), req, resp)
}

func (r *cloudconnectAwsAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_cloudconnect_aws_account.CloudconnectAwsAccountResourceSchema(ctx)

	// account_id: Required, ForceNew, UseStateForUnknown (stable after create).
	if attr, ok := s.Attributes["account_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			stringplanmodifier.RequiresReplace(),
			stringplanmodifier.UseStateForUnknown(),
		)
		s.Attributes["account_id"] = attr
	}

	// role_arn: Required, ForceNew — the Update API does not support changing
	// the role ARN, so a new value requires resource recreation.
	if attr, ok := s.Attributes["role_arn"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			stringplanmodifier.RequiresReplace(),
		)
		s.Attributes["role_arn"] = attr
	}

	// time_linked: Computed-only, UseStateForUnknown (stable after create).
	if attr, ok := s.Attributes["time_linked"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["time_linked"] = attr
	}

	// supported_features: Computed-only list, UseStateForUnknown.
	if attr, ok := s.Attributes["supported_features"].(schema.ListNestedAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, listplanmodifier.UseStateForUnknown())
		s.Attributes["supported_features"] = attr
	}

	// s3bucket and s3bucket_region are mutually dependent — validate at plan time.
	// Clearable (Category A): the user clears S3 by removing "real-time-data"
	// from enabled_features and omitting these fields. The API returns "" after
	// clearing, so useEmptyForUnknownWhenConfigNull() avoids null↔"" drift.
	if attr, ok := s.Attributes["s3bucket"].(schema.StringAttribute); ok {
		attr.Validators = append(attr.Validators, stringvalidator.AlsoRequires(path.MatchRoot("s3bucket_region")))
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["s3bucket"] = attr
	}
	if attr, ok := s.Attributes["s3bucket_region"].(schema.StringAttribute); ok {
		attr.Validators = append(attr.Validators, stringvalidator.AlsoRequires(path.MatchRoot("s3bucket")))
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["s3bucket_region"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *cloudconnectAwsAccountResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		cloudconnectAwsS3RealTimeValidator{},
	}
}

func (r *cloudconnectAwsAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cloudconnectAwsAccountResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Build API request body from plan.
	apiReq := models.CreateAccountRoleRequestBody{
		AccountID: plan.AccountId.ValueString(),
		RoleArn:   plan.RoleArn.ValueString(),
	}

	// Extract enabledFeatures from plan.
	var enabledFeatures []string
	resp.Diagnostics.Append(plan.EnabledFeatures.ElementsAs(ctx, &enabledFeatures, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq.EnabledFeatures = enabledFeatures

	if !plan.S3bucket.IsNull() && !plan.S3bucket.IsUnknown() {
		apiReq.S3Bucket = plan.S3bucket.ValueStringPointer()
	}
	if !plan.S3bucketRegion.IsNull() && !plan.S3bucketRegion.IsUnknown() {
		apiReq.S3BucketRegion = plan.S3bucketRegion.ValueStringPointer()
	}

	// Create new AWS account role via API.
	createResp, err := r.client.CreateAccountRoleWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating CloudConnect AWS Account",
			"Could not create CloudConnect AWS account, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Creating CloudConnect AWS Account",
			fmt.Sprintf("Could not create CloudConnect AWS account, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Creating CloudConnect AWS Account",
			"Could not create CloudConnect AWS account, empty response",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from API response.
	resp.Diagnostics.Append(overlayCloudConnectAwsAccountComputedFields(ctx, createResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudconnectAwsAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cloudconnectAwsAccountResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// Get refreshed AWS account from API.
	accountResp, err := r.client.GetAwsAccountWithResponse(ctx, state.AccountId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading CloudConnect AWS Account",
			"Could not read CloudConnect AWS account ID "+state.AccountId.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle externally deleted resource — remove from state.
	if accountResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check for successful response.
	if accountResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading CloudConnect AWS Account",
			fmt.Sprintf("Unexpected status code %d for CloudConnect AWS account ID %s: %s", accountResp.StatusCode(), state.AccountId.ValueString(), string(accountResp.Body)),
		)
		return
	}

	if accountResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading CloudConnect AWS Account",
			"Received empty response body for CloudConnect AWS account ID "+state.AccountId.ValueString(),
		)
		return
	}

	// Map response to state (full mapping — no overlay needed for Read).
	resp.Diagnostics.Append(mapCloudConnectAwsAccountToModel(ctx, accountResp.JSON200, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cloudconnectAwsAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cloudconnectAwsAccountResourceModel

	// Read Terraform plan data.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Build API request body — Update sends enabledFeatures and optional s3bucket fields.
	apiReq := models.UpdateAwsFeatureRequestBody{}

	// Extract enabledFeatures from plan.
	var enabledFeatures []string
	resp.Diagnostics.Append(plan.EnabledFeatures.ElementsAs(ctx, &enabledFeatures, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq.EnabledFeatures = enabledFeatures

	if !plan.S3bucket.IsNull() && !plan.S3bucket.IsUnknown() {
		apiReq.S3Bucket = plan.S3bucket.ValueStringPointer()
	}
	if !plan.S3bucketRegion.IsNull() && !plan.S3bucketRegion.IsUnknown() {
		apiReq.S3BucketRegion = plan.S3bucketRegion.ValueStringPointer()
	}

	// Update AWS account feature via API.
	updateResp, err := r.client.UpdateAwsFeatureWithResponse(ctx, plan.AccountId.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating CloudConnect AWS Account",
			"Could not update CloudConnect AWS account, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating CloudConnect AWS Account",
			fmt.Sprintf("Could not update CloudConnect AWS account, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating CloudConnect AWS Account",
			"Received empty response body",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from API response.
	resp.Diagnostics.Append(overlayCloudConnectAwsAccountComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudconnectAwsAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cloudconnectAwsAccountResourceModel

	// Read Terraform prior state data into the model.
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// Delete AWS account role via API.
	deleteResp, err := r.client.DeleteAccountRoleWithResponse(ctx, state.AccountId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting CloudConnect AWS Account",
			"Could not delete CloudConnect AWS account, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success — resource is already gone (deleted outside Terraform).
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting CloudConnect AWS Account",
			fmt.Sprintf("Could not delete CloudConnect AWS account, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
