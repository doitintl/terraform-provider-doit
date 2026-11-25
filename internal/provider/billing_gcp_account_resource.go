package provider

import (
	"context"
	"fmt"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_billing_gcp_account"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type billingGcpAccountResource struct {
	client *modelsv2.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*billingGcpAccountResource)(nil)
	_ resource.ResourceWithConfigure   = (*billingGcpAccountResource)(nil)
	_ resource.ResourceWithImportState = (*billingGcpAccountResource)(nil)
)

// NewBillingGcpAccountResource creates a new GCP billing account connection resource instance.
func NewBillingGcpAccountResource() resource.Resource {
	return &billingGcpAccountResource{}
}

// Configure builds the v2 API client from the provider configured client.
func (r *billingGcpAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	v2Client, err := newV2Client(client)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Could not create the v2 API client: "+err.Error()+". Please report this issue to the provider developers.",
		)
		return
	}

	r.client = v2Client
}

func (r *billingGcpAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_billing_gcp_account"
}

// ImportState imports a billing account connection by its billing account ID.
func (r *billingGcpAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *billingGcpAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_billing_gcp_account.BillingGcpAccountResourceSchema(ctx)

	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		attr.Description = "The ID of the connection, equal to `billing_account_id`."
		attr.MarkdownDescription = attr.Description
		s.Attributes["id"] = attr
	}

	// The API has no update: the billing account and the export are changed by
	// disconnecting and connecting again.
	for _, name := range []string{"billing_account_id", "export_project_id", "export_dataset_id"} {
		if attr, ok := s.Attributes[name].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
			s.Attributes[name] = attr
		}
	}

	if attr, ok := s.Attributes["status"].(schema.StringAttribute); ok {
		attr.Description = "Activation state: `pending` while DoiT activates the connection in the background, `active` once billing import is set up, `failed` (see `failure_reason`) when the activation or a disconnect did not finish. The provider does not wait for `active`; the next refresh reads the current state."
		attr.MarkdownDescription = attr.Description
		s.Attributes["status"] = attr
	}

	s.Description = "Connects a Google Cloud billing account to DoiT, so DoiT imports its billing data from the detailed usage cost export in BigQuery.\n\n" +
		"Before connecting, grant the DoiT service account from `doit_billing_gcp_requirements` the roles it lists, and enable the detailed usage cost export (Google offers no API for that step). " +
		"While granted access propagates, the API refuses the connection; the provider retries until the create timeout ends.\n\n" +
		"Creating the resource records the connection as `pending` and returns; DoiT activates it in the background, which takes up to about 40 minutes. The provider does not wait for that. Read `status` and `failure_reason` to follow it.\n\n" +
		"~> **Note:** Create adopts an existing `pending` or `active` connection of the same billing account and export, including one made in the DoiT console. A later `terraform destroy` then disconnects that billing account, even though Terraform did not create the connection. Two configurations must not manage the same billing account.\n\n" +
		"~> **Note:** The connection cannot be disconnected while it is still activating. Destroying or replacing it in that time fails; retry when `status` is `active` or `failed`. Disconnecting leaves the IAM grants and the export dataset in place."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Delete: true,
	})

	rejectNullCollectionElements(s.Attributes)
	resp.Schema = s
}

func (r *billingGcpAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan billingGcpAccountResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, DefaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	apiReq := plan.toCreateRequest()

	// A module grants DoiT's service account access seconds before it connects,
	// so the two errors for access that has not propagated yet are retried.
	var createResp *modelsv2.CreateGcpBillingAccountResp
	err := retryOnboardingRequest(ctx, []string{billingAccountAccessMissingCode, billingExportNotReadyCode}, func() error {
		params := &modelsv2.CreateGcpBillingAccountParams{
			IdempotencyKey: uuid.NewV4().String(),
		}
		var callErr error
		createResp, callErr = r.client.CreateGcpBillingAccountWithResponse(ctx, params, apiReq)
		return callErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating GCP Billing Account",
			"Could not connect GCP billing account, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating GCP Billing Account",
			fmt.Sprintf("Could not connect GCP billing account, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating GCP Billing Account",
			"Received empty response body for connected GCP billing account",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from the API response.
	overlayBillingGcpAccountComputedFields(createResp.JSON201, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *billingGcpAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state billingGcpAccountResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Disconnected outside Terraform
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only runs when timeouts change: every other configurable attribute
// requires replacement. It sends nothing and refreshes the connection.
func (r *billingGcpAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan billingGcpAccountResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, DefaultUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	var state billingGcpAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.GetGcpBillingAccountWithResponse(ctx, state.Id.ValueString(), &modelsv2.GetGcpBillingAccountParams{})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating GCP Billing Account",
			"Could not read GCP billing account connection, unexpected error: "+err.Error(),
		)
		return
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating GCP Billing Account",
			fmt.Sprintf("Could not read GCP billing account connection, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from the API response.
	overlayBillingGcpAccountComputedFields(getResp.JSON200, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *billingGcpAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state billingGcpAccountResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// Another connect or disconnect of the same billing account is retried.
	var deleteResp *modelsv2.DeleteGcpBillingAccountResp
	err := retryOnboardingRequest(ctx, []string{billingAccountBusyCode}, func() error {
		params := &modelsv2.DeleteGcpBillingAccountParams{
			IdempotencyKey: uuid.NewV4().String(),
		}
		var callErr error
		deleteResp, callErr = r.client.DeleteGcpBillingAccountWithResponse(ctx, state.Id.ValueString(), params)
		return callErr
	})
	if err != nil {
		detail := "Could not disconnect GCP billing account, unexpected error: " + err.Error()
		if problem, ok := onboardingProblemFromError(err); ok && problem.Code == billingConnectionActivatingCode {
			detail += "\n\nThe connection is still activating, which takes up to about 40 minutes after it was connected, and cannot be disconnected until then. Retry when the connection is active or failed."
		}
		resp.Diagnostics.AddError("Error Deleting GCP Billing Account", detail)
		return
	}

	// Treat 404 as success: the connection is already gone.
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting GCP Billing Account",
			fmt.Sprintf("Could not disconnect GCP billing account, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
	}
}
