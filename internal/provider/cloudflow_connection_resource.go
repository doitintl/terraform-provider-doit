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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                     = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithConfigure        = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithImportState      = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithConfigValidators = (*cloudflowConnectionResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*cloudflowConnectionResource)(nil)
)

var (
	cloudflowConnectionEmailPattern     = regexp.MustCompile(`^[^@\s<>"]+@[^@\s<>"]+\.[^@\s<>"]+$`)
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
		cloudflowConnectionAwsConfigValidator{},
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

	emptyStrings := listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{}))
	emptyActions := listdefault.StaticValue(types.ListValueMust(resource_cloudflow_connection.ActionsValue{}.Type(ctx), []attr.Value{}))
	notEmpty := stringvalidator.LengthAtLeast(1)
	noEmptyItems := listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))

	// Stable computed fields keep their value across updates.
	if a, ok := s.Attributes["connection_id"].(schema.StringAttribute); ok {
		a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["connection_id"] = a
	}
	if a, ok := s.Attributes["created_at"].(schema.StringAttribute); ok {
		a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["created_at"] = a
	}

	if a, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		a.Validators = append(a.Validators, notEmpty)
		s.Attributes["name"] = a
	}

	// The description is user-authored. Removing it from config clears it (sent
	// as an empty string, which the API echoes back and the mapper reads as null).
	if a, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		a.Computed = false
		a.Validators = append(a.Validators, notEmpty)
		s.Attributes["description"] = a
	}

	// Collaborators are always declared explicitly: the API adds the caller as
	// owner when none is listed, which would make an omitted or owner-less list
	// differ from the plan. Exactly one owner is required.
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
			b.Validators = append(b.Validators, stringvalidator.OneOf("owner", "editor", "user"))
			a.NestedObject.Attributes["role"] = b
		}
		s.Attributes["collaborators"] = a
	}

	// The configuration blocks are the source of truth: the API replaces them as
	// a whole, so removing a value from config clears it. Optional lists default
	// to [] (the API omits empty lists); optional strings are null when unset.
	if a, ok := s.Attributes["gcp_config"].(schema.SingleNestedAttribute); ok {
		a.Computed = false
		if b, ok := a.Attributes["level"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			b.Validators = append(b.Validators, stringvalidator.OneOf("organization", "folder", "project"))
			a.Attributes["level"] = b
		}
		if b, ok := a.Attributes["service_account_name"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["service_account_name"] = b
		}
		if b, ok := a.Attributes["organization_id"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["organization_id"] = b
		}
		if b, ok := a.Attributes["folder_id"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["folder_id"] = b
		}
		if b, ok := a.Attributes["project_id"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["project_id"] = b
		}
		if b, ok := a.Attributes["infra_manager_project"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["infra_manager_project"] = b
		}
		if b, ok := a.Attributes["infra_manager_location"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["infra_manager_location"] = b
		}
		if b, ok := a.Attributes["infra_manager_service_account"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["infra_manager_service_account"] = b
		}
		if b, ok := a.Attributes["predefined_roles"].(schema.ListAttribute); ok {
			b.Default = emptyStrings
			b.Validators = append(b.Validators, noEmptyItems)
			a.Attributes["predefined_roles"] = b
		}
		if b, ok := a.Attributes["custom_role"].(schema.SingleNestedAttribute); ok {
			b.Computed = false
			if c, ok := b.Attributes["role_id"].(schema.StringAttribute); ok {
				c.Required = true
				c.Optional = false
				c.Computed = false
				c.Validators = append(c.Validators, notEmpty)
				b.Attributes["role_id"] = c
			}
			if c, ok := b.Attributes["permissions"].(schema.ListAttribute); ok {
				c.Default = emptyStrings
				c.Validators = append(c.Validators, noEmptyItems)
				b.Attributes["permissions"] = c
			}
			a.Attributes["custom_role"] = b
		}
		s.Attributes["gcp_config"] = a
	}

	if a, ok := s.Attributes["aws_config"].(schema.SingleNestedAttribute); ok {
		a.Computed = false
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
				c.Default = emptyStrings
				c.Validators = append(c.Validators, noEmptyItems)
				b.NestedObject.Attributes["regions"] = c
			}
			a.Attributes["context"] = b
		}
		if b, ok := a.Attributes["role_name"].(schema.StringAttribute); ok {
			b.Required = true
			b.Optional = false
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["role_name"] = b
		}
		if b, ok := a.Attributes["management_account"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["management_account"] = b
		}
		if b, ok := a.Attributes["organization_root_id"].(schema.StringAttribute); ok {
			b.Computed = false
			b.Validators = append(b.Validators, notEmpty)
			a.Attributes["organization_root_id"] = b
		}
		if b, ok := a.Attributes["scope_excluded_account_ids"].(schema.ListAttribute); ok {
			b.Default = emptyStrings
			b.Validators = append(b.Validators, noEmptyItems)
			a.Attributes["scope_excluded_account_ids"] = b
		}
		if b, ok := a.Attributes["scope_explicit_account_ids"].(schema.ListAttribute); ok {
			b.Default = emptyStrings
			b.Validators = append(b.Validators, noEmptyItems)
			a.Attributes["scope_explicit_account_ids"] = b
		}
		if b, ok := a.Attributes["scope_targeted_organizational_unit_ids"].(schema.ListAttribute); ok {
			b.Default = emptyStrings
			b.Validators = append(b.Validators, noEmptyItems)
			a.Attributes["scope_targeted_organizational_unit_ids"] = b
		}
		if b, ok := a.Attributes["scope_management_account_explicit_in_scope"].(schema.BoolAttribute); ok {
			b.Default = booldefault.StaticBool(false)
			a.Attributes["scope_management_account_explicit_in_scope"] = b
		}
		if b, ok := a.Attributes["permissions"].(schema.SingleNestedAttribute); ok {
			b.Computed = false
			if c, ok := b.Attributes["actions"].(schema.ListNestedAttribute); ok {
				c.Default = emptyActions
				if d, ok := c.NestedObject.Attributes["value"].(schema.StringAttribute); ok {
					d.Validators = append(d.Validators, notEmpty)
					c.NestedObject.Attributes["value"] = d
				}
				b.Attributes["actions"] = c
			}
			if c, ok := b.Attributes["aws_managed_policies"].(schema.ListAttribute); ok {
				c.Default = emptyStrings
				c.Validators = append(c.Validators, noEmptyItems)
				b.Attributes["aws_managed_policies"] = c
			}
			a.Attributes["permissions"] = b
		}
		s.Attributes["aws_config"] = a
	}

	setCloudflowConnectionDescriptions(s.Attributes)

	s.Description = "Manages a CloudFlow connection: a GCP or AWS configuration that CloudFlow workflows use to act on a cloud account.\n\n" +
		"Set exactly one of `gcp_config` or `aws_config`; switching between them replaces the connection. " +
		"Updates send the complete configuration, so anything removed from the configuration block is cleared.\n\n" +
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

// setCloudflowConnectionDescriptions replaces the generated, API-oriented
// descriptions with ones written for Terraform users.
func setCloudflowConnectionDescriptions(attrs map[string]schema.Attribute) {
	describe := func(a map[string]schema.Attribute, descriptions map[string]string) {
		for name, description := range descriptions {
			switch v := a[name].(type) {
			case schema.StringAttribute:
				v.Description, v.MarkdownDescription = description, description
				a[name] = v
			case schema.BoolAttribute:
				v.Description, v.MarkdownDescription = description, description
				a[name] = v
			case schema.ListAttribute:
				v.Description, v.MarkdownDescription = description, description
				a[name] = v
			case schema.ListNestedAttribute:
				v.Description, v.MarkdownDescription = description, description
				a[name] = v
			case schema.SingleNestedAttribute:
				v.Description, v.MarkdownDescription = description, description
				a[name] = v
			default:
				panic(fmt.Sprintf("setCloudflowConnectionDescriptions: unsupported attribute %q (%T)", name, v))
			}
		}
	}

	describe(attrs, map[string]string{
		"connection_id": "The ID of the connection.",
		"name":          "Display name of the connection.",
		"description":   "Description of the connection. Remove it to clear it.",
		"enabled":       "Whether the connection is enabled. Defaults to `true`. Set to `false` to disable it without deleting it.",
		"collaborators": "Users with access to the connection. Exactly one must be the `owner`, and the user behind the API token must be listed. " +
			"Roles: `owner` can update, delete and transfer the connection, `editor` can update it, `user` has read access.",
		"status":     "Overall status of the connection, as reported by the API.",
		"etag":       "Current version of the connection. The provider sends it with updates and deletes to detect changes made outside Terraform.",
		"created_at": "When the connection was created (RFC 3339, UTC).",
		"updated_at": "When the connection was last changed (RFC 3339, UTC).",
		"gcp_config": "GCP configuration. Set exactly one of `gcp_config` or `aws_config`. Changing between them replaces the connection.",
		"aws_config": "AWS configuration. Set exactly one of `gcp_config` or `aws_config`. Changing between them replaces the connection.",
	})

	if gcp, ok := attrs["gcp_config"].(schema.SingleNestedAttribute); ok {
		describe(gcp.Attributes, map[string]string{
			"level":                         "Scope of the connection: `organization`, `folder` or `project`. The matching `organization_id`, `folder_id` or `project_id` is required.",
			"organization_id":               "ID of the GCP organization. Required when `level` is `organization`.",
			"folder_id":                     "ID of the GCP folder. Required when `level` is `folder`.",
			"project_id":                    "ID of the GCP project. Required when `level` is `project`. The project must be known to the DoiT customer.",
			"service_account_name":          "Name of the service account CloudFlow uses in the GCP project.",
			"predefined_roles":              "Predefined GCP roles granted to the service account, such as `roles/viewer`.",
			"custom_role":                   "Custom GCP role granted to the service account.",
			"infra_manager_project":         "GCP project that runs Infrastructure Manager for the deployment.",
			"infra_manager_location":        "Location of the Infrastructure Manager deployment.",
			"infra_manager_service_account": "Service account Infrastructure Manager uses for the deployment.",
			"status":                        "Status of the GCP configuration, as reported by the API.",
			"deployment_command":            "Command that deploys the connection's resources in GCP, as generated by the API.",
		})
		if customRole, ok := gcp.Attributes["custom_role"].(schema.SingleNestedAttribute); ok {
			describe(customRole.Attributes, map[string]string{
				"role_id":     "ID of the custom role.",
				"permissions": "Permissions of the custom role.",
			})
		}
	}

	if aws, ok := attrs["aws_config"].(schema.SingleNestedAttribute); ok {
		describe(aws.Attributes, map[string]string{
			"context":                                    "AWS accounts the connection covers, with the regions for each.",
			"role_name":                                  "Name of the IAM role CloudFlow assumes. Must be unique per AWS account.",
			"permissions":                                "IAM permissions granted to the role. Set at least one of `actions` or `aws_managed_policies`, or remove the block.",
			"management_account":                         "ID of the AWS organization's management account.",
			"organization_root_id":                       "ID of the AWS organization root.",
			"scope_excluded_account_ids":                 "AWS account IDs excluded from the connection's scope.",
			"scope_explicit_account_ids":                 "AWS account IDs explicitly included in the connection's scope.",
			"scope_targeted_organizational_unit_ids":     "IDs of the organizational units the connection targets.",
			"scope_management_account_explicit_in_scope": "Whether the management account is explicitly in scope. Defaults to `false`.",
		})
		if awsContext, ok := aws.Attributes["context"].(schema.ListNestedAttribute); ok {
			describe(awsContext.NestedObject.Attributes, map[string]string{
				"account_id": "12-digit AWS account ID.",
				"regions":    "AWS regions of the account in scope.",
				"status":     "Deployment status of the account, as reported by the API.",
			})
		}
		if permissions, ok := aws.Attributes["permissions"].(schema.SingleNestedAttribute); ok {
			describe(permissions.Attributes, map[string]string{
				"actions":              "Custom IAM actions granted to the role.",
				"aws_managed_policies": "ARNs of AWS-managed policies attached to the role.",
			})
			if actions, ok := permissions.Attributes["actions"].(schema.ListNestedAttribute); ok {
				describe(actions.NestedObject.Attributes, map[string]string{
					"value": "IAM action, for example `s3:ListBucket`.",
					"id":    "Identifier of the action, as reported by the API when it returns one.",
				})
			}
		}
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
