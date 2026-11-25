package provider

import (
	"context"
	"fmt"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_gcp_connection"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type cloudconnectGcpConnectionResource struct {
	client *modelsv2.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*cloudconnectGcpConnectionResource)(nil)
	_ resource.ResourceWithConfigure        = (*cloudconnectGcpConnectionResource)(nil)
	_ resource.ResourceWithImportState      = (*cloudconnectGcpConnectionResource)(nil)
	_ resource.ResourceWithConfigValidators = (*cloudconnectGcpConnectionResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*cloudconnectGcpConnectionResource)(nil)
)

// NewCloudconnectGcpConnectionResource creates a new GCP Cloud Connect connection resource instance.
func NewCloudconnectGcpConnectionResource() resource.Resource {
	return &cloudconnectGcpConnectionResource{}
}

// Configure builds the v2 API client from the provider configured client.
func (r *cloudconnectGcpConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *cloudconnectGcpConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudconnect_gcp_connection"
}

// ImportState imports a connection by its ID.
func (r *cloudconnectGcpConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *cloudconnectGcpConnectionResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_cloudconnect_gcp_connection.CloudconnectGcpConnectionResourceSchema(ctx)

	// Stable Computed-only attributes.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["binding_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["binding_id"] = attr
	}
	if attr, ok := s.Attributes["registered_via"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["registered_via"] = attr
	}
	if attr, ok := s.Attributes["registration"].(schema.SingleNestedAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, objectplanmodifier.UseStateForUnknown())
		s.Attributes["registration"] = attr
	}

	// The scope and the service account cannot change after creation.
	if attr, ok := s.Attributes["scope_kind"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["scope_kind"] = attr
	}
	if attr, ok := s.Attributes["service_account"].(schema.SingleNestedAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, objectplanmodifier.RequiresReplace())
		s.Attributes["service_account"] = attr
	}
	// The scope IDs and role_id are Optional+Computed: omitted, they read back
	// as the API's value (null for an ID the scope does not use, the default
	// role), so only a configured value can require replacement.
	for _, name := range []string{"organization_id", "project_id", "dataset_id", "role_id"} {
		if attr, ok := s.Attributes[name].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplaceIfConfigured())
			s.Attributes[name] = attr
		}
	}

	// feature_inputs is cleared by omitting it (featureInputs: null). The read
	// returns it only with a location, so a configured block must carry one.
	if attr, ok := s.Attributes["feature_inputs"].(schema.SingleNestedAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownGcpFeatureInputsWhenConfigNull())
		attr.Validators = append(attr.Validators, objectvalidator.AlsoRequires(path.MatchRelative().AtName("bigquery_finops")))
		if finops, ok := attr.Attributes["bigquery_finops"].(schema.SingleNestedAttribute); ok {
			finops.Validators = append(finops.Validators, objectvalidator.AlsoRequires(path.MatchRelative().AtName("location")))
			finops.Description = "Inputs of the `bigquery-finops` feature. Requires `bigquery-finops` in `feature_ids`."
			finops.MarkdownDescription = finops.Description
			attr.Attributes["bigquery_finops"] = finops
		}
		attr.Description = "Inputs of features that need them. Omit it to use the API defaults; removing it clears the stored inputs."
		attr.MarkdownDescription = attr.Description
		s.Attributes["feature_inputs"] = attr
	}

	acknowledgeNotClearable(s,
		"organization_id",                // immutable; null unless the scope uses it
		"project_id",                     // immutable; null unless the scope uses it
		"dataset_id",                     // immutable; null unless the scope uses it
		"role_id",                        // immutable; the API defaults it
		"catalog_version",                // the API records the current catalog version on every write
		"feature_inputs.bigquery_finops", // required by its validator while feature_inputs is set
		"feature_inputs.bigquery_finops.location", // required by its validator while bigquery_finops is set
	)

	setCostAnomalyDescriptions(s.Attributes, map[string]string{
		"id":                "The ID of the connection.",
		"etag":              "Current ETag of the connection. The provider sends it with updates and deletes to detect changes made outside Terraform.",
		"revision":          "Configuration revision of the connection.",
		"registered_via":    "How the connection was registered: `api`, `console` or `cloudformation_callback`.",
		"client_user_agent": "User-Agent of the client that last changed the connection.",
		"binding_id":        "Key that relates `follow_ups` entries to this connection.",
		"scope_kind":        "Google Cloud resource the connection grants access to: `gcp_organization`, `gcp_project` or `gcp_dataset`. Changing it replaces the connection.",
		"organization_id":   "Numeric Google Cloud organization ID. Required when `scope_kind` is `gcp_organization`. Changing it replaces the connection.",
		"project_id":        "Google Cloud project ID. Required when `scope_kind` is `gcp_project` or `gcp_dataset`. Changing it replaces the connection.",
		"dataset_id":        "BigQuery dataset ID. Required when `scope_kind` is `gcp_dataset`. Changing it replaces the connection.",
		"role_id":           "ID of the custom IAM role created for the connection. Defaults to `doit_cmp_role` for an organization or project and to a dataset role for a dataset. Changing it replaces the connection.",
		"service_account":   "The customer's Google Cloud service account that DoiT impersonates. One service account can hold only one connection. Changing it replaces the connection.",
		"catalog_version":   "Catalog version from `doit_cloudconnect_gcp_requirements`. When set, the API refuses the change if the feature catalog changed since. When omitted, the provider stores the version the API recorded.",
	})
	if attr, ok := s.Attributes["feature_ids"].(schema.SetAttribute); ok {
		attr.Description = "Cloud Connect features to enable. Valid IDs depend on the customer and the scope; `doit_cloudconnect_gcp_requirements` lists them."
		attr.MarkdownDescription = attr.Description
		s.Attributes["feature_ids"] = attr
	}

	s.Description = "Registers a Google Cloud organization, project or BigQuery dataset with DoiT Cloud Connect, so DoiT can read it through a customer service account it impersonates.\n\n" +
		"Use `doit_cloudconnect_gcp_requirements` to find the service account placement, the IAM permissions and the feature IDs for the scope. " +
		"While IAM changes propagate (usually 2 to 7 minutes), the API refuses the registration; the provider retries until the create or update timeout ends, so raise `timeouts` if propagation takes longer.\n\n" +
		"~> **Note:** One Google Cloud service account can hold only one connection. A second connection with the same service account fails with `409 onboarding.connection_exists`.\n\n" +
		"Destroying the resource removes DoiT's record only; the service account and its IAM bindings stay in place."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	rejectNullCollectionElements(s.Attributes)
	resp.Schema = s
}

// ModifyPlan marks the attributes the API changes on every write unknown on
// updates, including one that only clears feature_inputs.
func (r *cloudconnectGcpConnectionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp,
		path.Root("etag"),
		path.Root("revision"),
		path.Root("client_user_agent"),
		path.Root("follow_ups"),
		path.Root("baseline_checks"),
		path.Root("feature_verification"),
	)
}

func (r *cloudconnectGcpConnectionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		cloudconnectGcpFeatureInputsValidator{},
	}
}

func (r *cloudconnectGcpConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cloudconnectGcpConnectionResourceModel

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

	apiReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// IAM changes made in the same apply take minutes to propagate; the API
	// marks the gate failures they cause as retryable.
	var createResp *modelsv2.CreateGcpCloudConnectConnectionResp
	err := retryOnboardingRequest(ctx, []string{registrationGateFailedCode}, func() error {
		params := &modelsv2.CreateGcpCloudConnectConnectionParams{
			IdempotencyKey: uuid.NewV4().String(),
		}
		var callErr error
		createResp, callErr = r.client.CreateGcpCloudConnectConnectionWithResponse(ctx, params, apiReq)
		return callErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating GCP Cloud Connect Connection",
			"Could not register GCP Cloud Connect connection, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating GCP Cloud Connect Connection",
			fmt.Sprintf("Could not register GCP Cloud Connect connection, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating GCP Cloud Connect Connection",
			"Received empty response body for registered GCP Cloud Connect connection",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from the API response.
	resp.Diagnostics.Append(overlayCloudconnectGcpConnectionComputedFields(ctx, createResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	resp.Diagnostics.Append(cloudconnectGcpFollowUpWarnings(createResp.JSON201.FollowUps)...)
}

func (r *cloudconnectGcpConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cloudconnectGcpConnectionResourceModel

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

func (r *cloudconnectGcpConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cloudconnectGcpConnectionResourceModel

	// Read Terraform plan data
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

	// Get the ID and ETag from the prior state
	var state cloudconnectGcpConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := plan.toUpdateRequest(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Enabling a feature can need IAM permissions granted in the same apply.
	var updateResp *modelsv2.UpdateGcpCloudConnectConnectionResp
	err := retryOnboardingRequest(ctx, []string{registrationGateFailedCode}, func() error {
		params := &modelsv2.UpdateGcpCloudConnectConnectionParams{
			IfMatch:        state.Etag.ValueString(),
			IdempotencyKey: uuid.NewV4().String(),
		}
		var callErr error
		updateResp, callErr = r.client.UpdateGcpCloudConnectConnectionWithApplicationMergePatchPlusJSONBodyWithResponse(ctx, state.Id.ValueString(), params, apiReq)
		return callErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating GCP Cloud Connect Connection",
			"Could not update GCP Cloud Connect connection, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating GCP Cloud Connect Connection",
			fmt.Sprintf("Could not update GCP Cloud Connect connection, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating GCP Cloud Connect Connection",
			"Received empty response body for updated GCP Cloud Connect connection",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from the API response.
	resp.Diagnostics.Append(overlayCloudconnectGcpConnectionComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	resp.Diagnostics.Append(cloudconnectGcpFollowUpWarnings(updateResp.JSON200.FollowUps)...)
}

func (r *cloudconnectGcpConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cloudconnectGcpConnectionResourceModel

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

	id := state.Id.ValueString()

	etag := state.Etag.ValueString()
	if etag == "" {
		// A connection that is already gone is treated as deleted.
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		etag = current
	}

	params := &modelsv2.DeleteGcpCloudConnectConnectionParams{
		CleanupScope:   modelsv2.DeleteGcpCloudConnectConnectionParamsCleanupScopeDoitOnly,
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteGcpCloudConnectConnectionWithResponse(ctx, id, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting GCP Cloud Connect Connection",
			"Could not disconnect GCP Cloud Connect connection, unexpected error: "+err.Error(),
		)
		return
	}

	// If 412 (stale ETag), retry once with a fresh ETag
	if deleteResp.StatusCode() == 412 {
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		params.IfMatch = current
		params.IdempotencyKey = uuid.NewV4().String()
		deleteResp, err = r.client.DeleteGcpCloudConnectConnectionWithResponse(ctx, id, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting GCP Cloud Connect Connection",
				"Could not disconnect GCP Cloud Connect connection on retry: "+err.Error(),
			)
			return
		}
	}

	// Treat 404 as success: the connection is already gone.
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting GCP Cloud Connect Connection",
			fmt.Sprintf("Could not disconnect GCP Cloud Connect connection, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
	}
}

// currentEtag fetches the connection's current ETag. found is false when the
// connection no longer exists.
func (r *cloudconnectGcpConnectionResource) currentEtag(ctx context.Context, id string) (etag string, found bool, diags diag.Diagnostics) {
	getResp, err := r.client.GetGcpCloudConnectConnectionWithResponse(ctx, id, &modelsv2.GetGcpCloudConnectConnectionParams{})
	if err != nil {
		diags.AddError(
			"Error Deleting GCP Cloud Connect Connection",
			"Could not fetch GCP Cloud Connect connection to get ETag: "+err.Error(),
		)
		return "", false, diags
	}
	if getResp.StatusCode() == 404 {
		return "", false, diags
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		diags.AddError(
			"Error Deleting GCP Cloud Connect Connection",
			fmt.Sprintf("Could not fetch GCP Cloud Connect connection to get ETag, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return "", false, diags
	}
	if getResp.JSON200.Etag != nil {
		etag = *getResp.JSON200.Etag
	}
	return etag, true, diags
}
