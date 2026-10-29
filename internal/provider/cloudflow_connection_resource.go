package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudflow_connection"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var (
	_ resource.Resource                     = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithConfigure        = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithImportState      = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithConfigValidators = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*cloudflowConnectionResource)(nil)
)

const (
	emailAtext = "[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+"
	emailLabel = "[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?"
)

var (
	// A plain address (dot-atom local part, dotted domain): no display name, list of
	// addresses, empty dot-separated parts or whitespace. The API accepts anything.
	cloudflowConnectionEmailPattern     = regexp.MustCompile("^" + emailAtext + `(?:\.` + emailAtext + ")*@" + emailLabel + `(?:\.` + emailLabel + ")+$")
	cloudflowConnectionAccountIDPattern = regexp.MustCompile(`^[0-9]{12}$`)
)

// NewCloudflowConnectionResource creates a new CloudFlow connection resource instance.
func NewCloudflowConnectionResource() resource.Resource {
	return &cloudflowConnectionResource{}
}

type cloudflowConnectionResource struct {
	client *models.ClientWithResponses
}

// Configure adds the provider configured client to the resource.
func (r *cloudflowConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *cloudflowConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudflow_connection"
}

func (r *cloudflowConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("connection_id"), req, resp)
}

func (r *cloudflowConnectionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("gcp_config"),
			path.MatchRoot("aws_config"),
		),
		cloudflowConnectionGcpConfigValidator{},
	}
}

// ModifyPlan forces replacement when the connection switches between a GCP and
// an AWS configuration: the API rejects changing the kind of an existing
// connection, so the switch becomes a destroy+create instead of an API error.
func (r *cloudflowConnectionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // create or destroy — nothing to compare
	}

	requiresReplaceWhenCleared[resource_cloudflow_connection.GcpConfigValue](ctx, req, resp, path.Root("gcp_config"))
	requiresReplaceWhenCleared[resource_cloudflow_connection.AwsConfigValue](ctx, req, resp, path.Root("aws_config"))
}

func (r *cloudflowConnectionResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_cloudflow_connection.CloudflowConnectionResourceSchema(ctx)

	// Stable computed fields keep their value across updates.
	if a, ok := s.Attributes["connection_id"].(schema.StringAttribute); ok {
		a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["connection_id"] = a
	}
	if a, ok := s.Attributes["created_at"].(schema.StringAttribute); ok {
		a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["created_at"] = a
	}

	// The API replaces gcpConfig, awsConfig and collaborators as a whole, so a value
	// removed from the configuration must be cleared (Category A) rather than kept.
	if a, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		a.PlanModifiers = append(a.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = a
	}

	// The API accepts a connection without collaborators (and without an owner) but then
	// denies everyone access to it, so collaborators are required, with exactly one owner.
	if a, ok := s.Attributes["collaborators"].(schema.ListNestedAttribute); ok {
		a.Required = true
		a.Optional = false
		a.Computed = false
		a.Validators = append(a.Validators, cloudflowConnectionCollaboratorsValidator{})
		if b, ok := a.NestedObject.Attributes["email"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			b.Validators = append(b.Validators, stringvalidator.RegexMatches(cloudflowConnectionEmailPattern, "must be a plain email address without a display name"))
			a.NestedObject.Attributes["email"] = b
		}
		if b, ok := a.NestedObject.Attributes["role"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			a.NestedObject.Attributes["role"] = b
		}
		s.Attributes["collaborators"] = a
	}

	if a, ok := s.Attributes["gcp_config"].(schema.SingleNestedAttribute); ok {
		// The API rejects a configuration without these (see CMP-55336), although the spec marks them optional.
		if b, ok := a.Attributes["level"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			a.Attributes["level"] = b
		}
		if b, ok := a.Attributes["service_account_name"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			b.Validators = append(b.Validators, stringvalidator.LengthAtLeast(1))
			a.Attributes["service_account_name"] = b
		}
		if b, ok := a.Attributes["organization_id"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["organization_id"] = b
		}
		if b, ok := a.Attributes["folder_id"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["folder_id"] = b
		}
		if b, ok := a.Attributes["project_id"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["project_id"] = b
		}
		if b, ok := a.Attributes["infra_manager_project"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["infra_manager_project"] = b
		}
		if b, ok := a.Attributes["infra_manager_location"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["infra_manager_location"] = b
		}
		if b, ok := a.Attributes["infra_manager_service_account"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["infra_manager_service_account"] = b
		}
		if b, ok := a.Attributes["predefined_roles"].(schema.ListAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useNullForUnknownListWhenConfigNull())
			a.Attributes["predefined_roles"] = b
		}
		if b, ok := a.Attributes["custom_role"].(schema.SingleNestedAttribute); ok {
			// Not Required: the API always returns the custom role, and a Required leaf under
			// an Optional+Computed object that is always populated would plan as unknown on
			// every run. The config validator requires it when custom_role is set.
			if c, ok := b.Attributes["role_id"].(schema.StringAttribute); ok {
				c.PlanModifiers = append(c.PlanModifiers, useEmptyForUnknownWhenConfigNull())
				b.Attributes["role_id"] = c
			}
			if c, ok := b.Attributes["permissions"].(schema.ListAttribute); ok {
				c.PlanModifiers = append(c.PlanModifiers, useNullForUnknownListWhenConfigNull())
				b.Attributes["permissions"] = c
			}
			a.Attributes["custom_role"] = b
		}
		s.Attributes["gcp_config"] = a
	}

	if a, ok := s.Attributes["aws_config"].(schema.SingleNestedAttribute); ok {
		if b, ok := a.Attributes["context"].(schema.ListNestedAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			b.Validators = append(b.Validators, listvalidator.SizeAtLeast(1))
			if c, ok := b.NestedObject.Attributes["account_id"].(schema.StringAttribute); ok {
				c.Required = true
				c.Optional = false
				c.Computed = false
				c.Validators = append(c.Validators, stringvalidator.RegexMatches(cloudflowConnectionAccountIDPattern, "must be a 12-digit AWS account ID"))
				b.NestedObject.Attributes["account_id"] = c
			}
			if c, ok := b.NestedObject.Attributes["regions"].(schema.ListAttribute); ok {
				c.PlanModifiers = append(c.PlanModifiers, useNullForUnknownListWhenConfigNull())
				b.NestedObject.Attributes["regions"] = c
			}
			a.Attributes["context"] = b
		}
		if b, ok := a.Attributes["role_name"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			b.Validators = append(b.Validators, stringvalidator.LengthAtLeast(1))
			a.Attributes["role_name"] = b
		}
		if b, ok := a.Attributes["management_account"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["management_account"] = b
		}
		if b, ok := a.Attributes["organization_root_id"].(schema.StringAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			a.Attributes["organization_root_id"] = b
		}
		if b, ok := a.Attributes["scope_excluded_account_ids"].(schema.ListAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useNullForUnknownListWhenConfigNull())
			a.Attributes["scope_excluded_account_ids"] = b
		}
		if b, ok := a.Attributes["scope_explicit_account_ids"].(schema.ListAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useNullForUnknownListWhenConfigNull())
			a.Attributes["scope_explicit_account_ids"] = b
		}
		if b, ok := a.Attributes["scope_targeted_organizational_unit_ids"].(schema.ListAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useNullForUnknownListWhenConfigNull())
			a.Attributes["scope_targeted_organizational_unit_ids"] = b
		}
		if b, ok := a.Attributes["scope_management_account_explicit_in_scope"].(schema.BoolAttribute); ok {
			b.PlanModifiers = append(b.PlanModifiers, useEmptyForUnknownBoolWhenConfigNull())
			a.Attributes["scope_management_account_explicit_in_scope"] = b
		}
		if b, ok := a.Attributes["permissions"].(schema.SingleNestedAttribute); ok {
			if c, ok := b.Attributes["actions"].(schema.ListNestedAttribute); ok {
				c.PlanModifiers = append(c.PlanModifiers, useNullForUnknownListWhenConfigNull())
				b.Attributes["actions"] = c
			}
			if c, ok := b.Attributes["aws_managed_policies"].(schema.ListAttribute); ok {
				c.PlanModifiers = append(c.PlanModifiers, useNullForUnknownListWhenConfigNull())
				b.Attributes["aws_managed_policies"] = c
			}
			a.Attributes["permissions"] = b
		}
		s.Attributes["aws_config"] = a
	}

	// Category B: the API always returns these objects (an empty permissions object, an empty
	// custom role), so the object itself stays when its block is removed. Its values are
	// cleared by the modifiers above.
	acknowledgeNotClearable(s,
		"gcp_config.custom_role",
		"aws_config.permissions",
	)

	setCloudflowConnectionDescriptions(s.Attributes)

	s.Description = "Manages a CloudFlow connection: a GCP or AWS configuration that CloudFlow workflows use to act on a cloud account.\n\n" +
		"Set exactly one of `gcp_config` or `aws_config`; switching between them replaces the connection. " +
		"Updates send the complete configuration, so a value removed from it is cleared.\n\n" +
		"~> **Note:** The user behind the API token must be a collaborator of the connection, " +
		"otherwise the API denies access to it right after creating it. Declare that user as the `owner`."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

// setCloudflowConnectionDescriptions replaces the generated descriptions where
// they would mislead a Terraform user (the spec describes the API, not the resource).
func setCloudflowConnectionDescriptions(attrs map[string]schema.Attribute) {
	if a, ok := attrs["connection_id"].(schema.StringAttribute); ok {
		a.Description = "The ID of the connection."
		a.MarkdownDescription = a.Description
		attrs["connection_id"] = a
	}
	if a, ok := attrs["etag"].(schema.StringAttribute); ok {
		a.Description = "Current version of the connection. The provider sends it with updates and deletes to detect changes made outside Terraform."
		a.MarkdownDescription = a.Description
		attrs["etag"] = a
	}
	if a, ok := attrs["collaborators"].(schema.ListNestedAttribute); ok {
		a.Description = "Users with access to the connection. Exactly one must be the `owner`, and the user behind the API token must be listed. " +
			"Roles: `owner` can update, delete and transfer the connection, `editor` can update it, `user` has read access."
		a.MarkdownDescription = a.Description
		attrs["collaborators"] = a
	}
	if a, ok := attrs["gcp_config"].(schema.SingleNestedAttribute); ok {
		a.Description = "GCP configuration. Set exactly one of `gcp_config` or `aws_config`; switching between them replaces the connection. " +
			"Removing a value from it clears the value."
		a.MarkdownDescription = a.Description
		if p, ok := a.Attributes["project_id"].(schema.StringAttribute); ok {
			p.Description = "ID of the GCP project. Required when `level` is `project`, and the project must be known to the DoiT customer."
			p.MarkdownDescription = p.Description
			a.Attributes["project_id"] = p
		}
		attrs["gcp_config"] = a
	}
	if a, ok := attrs["aws_config"].(schema.SingleNestedAttribute); ok {
		a.Description = "AWS configuration. Set exactly one of `gcp_config` or `aws_config`; switching between them replaces the connection. " +
			"Removing a value from it clears the value."
		a.MarkdownDescription = a.Description
		if p, ok := a.Attributes["role_name"].(schema.StringAttribute); ok {
			p.Description = "Name of the IAM role CloudFlow assumes. Must be unique per AWS account."
			p.MarkdownDescription = p.Description
			a.Attributes["role_name"] = p
		}
		attrs["aws_config"] = a
	}
}

func (r *cloudflowConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cloudflowConnectionResourceModel

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

	params := &models.CreateCloudflowConnectionParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	createResp, err := r.client.CreateCloudflowConnectionWithResponse(ctx, params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating CloudFlow Connection",
			"Could not create CloudFlow connection, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating CloudFlow Connection",
			fmt.Sprintf("Could not create CloudFlow connection, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil || createResp.JSON201.ConnectionId == nil {
		resp.Diagnostics.AddError(
			"Error Creating CloudFlow Connection",
			"Received empty response body for created CloudFlow connection, body: "+string(createResp.Body),
		)
		return
	}

	data, diags := r.readAfterWrite(ctx, *createResp.JSON201.ConnectionId, newCloudflowConnectionData(createResp.JSON201, createResp.Body))
	resp.Diagnostics.Append(diags...)

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayCloudflowConnectionComputedFields(ctx, data, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudflowConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cloudflowConnectionResourceModel

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

	// Resource was deleted outside Terraform
	if state.ConnectionId.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cloudflowConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cloudflowConnectionResourceModel

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
	var state cloudflowConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ConnectionId.ValueString()

	updateResp, diags := r.updateConnection(ctx, id, state.Etag.ValueString(), apiReq)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating CloudFlow Connection",
			fmt.Sprintf("Could not update CloudFlow connection, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating CloudFlow Connection",
			"Received empty response body for updated CloudFlow connection, body: "+string(updateResp.Body),
		)
		return
	}

	data, diags := r.readAfterWrite(ctx, id, newCloudflowConnectionData(updateResp.JSON200, updateResp.Body))
	resp.Diagnostics.Append(diags...)

	resp.Diagnostics.Append(overlayCloudflowConnectionComputedFields(ctx, data, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudflowConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cloudflowConnectionResourceModel

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

	id := state.ConnectionId.ValueString()

	etag := state.Etag.ValueString()
	if etag == "" {
		// If state lacks an ETag, fetch it from the API
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		etag = current
	}

	params := &models.DeleteCloudflowConnectionParams{IfMatch: etag}

	deleteResp, err := r.client.DeleteCloudflowConnectionWithResponse(ctx, id, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting CloudFlow Connection",
			"Could not delete CloudFlow connection, unexpected error: "+err.Error()+cloudflowConnectionInUseHint(err.Error()),
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
		deleteResp, err = r.client.DeleteCloudflowConnectionWithResponse(ctx, id, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting CloudFlow Connection",
				"Could not delete CloudFlow connection on retry: "+err.Error(),
			)
			return
		}
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting CloudFlow Connection",
			fmt.Sprintf("Could not delete CloudFlow connection, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body))+
				cloudflowConnectionInUseHint(fmt.Sprintf("status: %d", deleteResp.StatusCode())),
		)
	}
}

// cloudflowConnectionInUseHint explains a 409 on delete: flows still reference the
// connection. The retry client reports a 409 as an error whose text carries the
// status, other clients return the response, so both spellings are recognized.
func cloudflowConnectionInUseHint(statusText string) string {
	if strings.Contains(statusText, "non-retryable error: 409") || strings.Contains(statusText, "status: 409") {
		return " The connection is in use by one or more flows. Remove it from those flows first."
	}
	return ""
}

// readAfterWrite returns the connection as GET reports it, falling back to the
// write response. GET is preferred because its etag and timestamps match what
// later reads return, which keeps state stable. It can be denied when the API
// token's user is not a collaborator of the connection; the write has succeeded
// by then, so the failure is reported as a warning instead of failing the apply.
func (r *cloudflowConnectionResource) readAfterWrite(ctx context.Context, id string, fallback *cloudflowConnectionData) (*cloudflowConnectionData, diag.Diagnostics) {
	fetched, found, diags := r.fetchConnection(ctx, id)
	if diags.HasError() {
		var warnings diag.Diagnostics
		for _, d := range diags.Errors() {
			warnings.AddWarning(
				"Could not read the CloudFlow connection after saving it",
				d.Detail()+" The connection was saved; its state comes from the save response. Make sure the user behind the API token is a collaborator.",
			)
		}
		return fallback, warnings
	}
	if !found {
		return fallback, diags
	}
	return fetched, diags
}

// updateConnection sends the update with the given ETag. When the API reports a
// stale ETag (412), it fetches the current one and retries once.
func (r *cloudflowConnectionResource) updateConnection(ctx context.Context, id, etag string, body models.UpdateCloudflowConnectionJSONRequestBody) (*models.UpdateCloudflowConnectionResp, diag.Diagnostics) {
	var diags diag.Diagnostics

	if etag == "" {
		current, found, d := r.currentEtag(ctx, id)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if !found {
			diags.AddError("Error Updating CloudFlow Connection", fmt.Sprintf("The CloudFlow connection %s no longer exists.", id))
			return nil, diags
		}
		etag = current
	}

	updateResp, err := r.client.UpdateCloudflowConnectionWithResponse(ctx, id, &models.UpdateCloudflowConnectionParams{IfMatch: etag}, body)
	if err != nil {
		diags.AddError("Error Updating CloudFlow Connection", "Could not update CloudFlow connection, unexpected error: "+err.Error())
		return nil, diags
	}

	if updateResp.StatusCode() == 412 {
		current, found, d := r.currentEtag(ctx, id)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if !found {
			diags.AddError("Error Updating CloudFlow Connection", fmt.Sprintf("The CloudFlow connection %s no longer exists.", id))
			return nil, diags
		}
		updateResp, err = r.client.UpdateCloudflowConnectionWithResponse(ctx, id, &models.UpdateCloudflowConnectionParams{IfMatch: current}, body)
		if err != nil {
			diags.AddError("Error Updating CloudFlow Connection", "Could not update CloudFlow connection on retry: "+err.Error())
			return nil, diags
		}
	}

	return updateResp, diags
}

// currentEtag fetches the connection's current ETag. found is false when the
// connection no longer exists.
func (r *cloudflowConnectionResource) currentEtag(ctx context.Context, id string) (etag string, found bool, diags diag.Diagnostics) {
	data, found, diags := r.fetchConnection(ctx, id)
	if diags.HasError() || !found {
		return "", found, diags
	}
	if data.conn.Etag != nil {
		etag = *data.conn.Etag
	}
	return etag, true, diags
}
