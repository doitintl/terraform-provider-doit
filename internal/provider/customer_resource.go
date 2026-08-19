package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*customerResource)(nil)
var _ resource.ResourceWithConfigure = (*customerResource)(nil)
var _ resource.ResourceWithImportState = (*customerResource)(nil)

func NewCustomerResource() resource.Resource {
	return &customerResource{}
}

// customerResource defines the resource implementation.
type customerResource struct {
	client *models.ClientWithResponses
}

type customerResourceModel struct {
	resource_customer.CustomerModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (r *customerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer"
}

func (r *customerResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_customer.CustomerResourceSchema(ctx)

	// Add UseStateForUnknown to stable Computed-only fields so they don't show
	// as unknown in update plans when other attributes change.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}

	if attr, ok := s.Attributes["customer_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			stringplanmodifier.UseStateForUnknown(),
			customerIDMatchesState(),
		)
		s.Attributes["customer_id"] = attr
	}

	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["name"] = attr
	}

	if attr, ok := s.Attributes["primary_domain"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["primary_domain"] = attr
	}

	if attr, ok := s.Attributes["domains"].(schema.ListAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, listplanmodifier.UseStateForUnknown())
		s.Attributes["domains"] = attr
	}

	if attr, ok := s.Attributes["url_slug"].(schema.StringAttribute); ok {
		attr.Validators = append(attr.Validators, customerURLSlug())
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["url_slug"] = attr
	}

	// Category A: Clearable nested attributes.
	if settingsAttr, ok := s.Attributes["settings"].(schema.SingleNestedAttribute); ok {
		if attr, ok := settingsAttr.Attributes["allowed_invite_domains"].(schema.ListAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
			settingsAttr.Attributes["allowed_invite_domains"] = attr
		}
		s.Attributes["settings"] = settingsAttr
	}

	if contactAttr, ok := s.Attributes["contact"].(schema.SingleNestedAttribute); ok {
		if attr, ok := contactAttr.Attributes["emails"].(schema.ListAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
			contactAttr.Attributes["emails"] = attr
		}
		s.Attributes["contact"] = contactAttr
	}

	// Classify Optional+Computed attributes (clearableattr).
	// Category B: Preserved settings when omitted from config.
	acknowledgeNotClearable(s,
		"customer_id",
		"settings",
		"settings.currency",
		"settings.mfa_required",
		"contact",
	)

	s.Description = "Manages customer general settings (URL slug, allowed invite domains, point-of-contact emails, currency) for the current tenant. " +
		"This resource is import-only: customer settings must first be imported into Terraform using `terraform import`. " +
		"Destroying this resource only removes it from Terraform state; the customer settings continue to exist in DoiT."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *customerResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"Customer Creation Not Supported",
		"The doit_customer resource does not support creation. "+
			"Customer settings already exist for your tenant and must be imported into Terraform state first.\n\n"+
			"To import the customer resource, run:\n"+
			"  terraform import doit_customer.<resource_name> <customer-id>",
	)
}

func (r *customerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customerResourceModel

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

	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customerResourceModel
	var state customerResourceModel
	var config customerResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
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

	customerID := ""
	if !state.Id.IsNull() && !state.Id.IsUnknown() {
		customerID = state.Id.ValueString()
	} else if !plan.Id.IsNull() && !plan.Id.IsUnknown() {
		customerID = plan.Id.ValueString()
	} else if !config.CustomerId.IsNull() && !config.CustomerId.IsUnknown() {
		customerID = config.CustomerId.ValueString()
	} else if !plan.CustomerId.IsNull() && !plan.CustomerId.IsUnknown() {
		customerID = plan.CustomerId.ValueString()
	}

	if customerID == "" {
		resp.Diagnostics.AddError(
			"Error Updating Customer",
			"Customer ID is required to update customer settings",
		)
		return
	}

	apiReq, d := plan.toUpdateRequest(ctx, &config)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.UpdateCustomerWithApplicationMergePatchPlusJSONBodyWithResponse(ctx, customerID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Customer",
			"Could not update customer, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Customer",
			fmt.Sprintf("Could not update customer, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	customerResp, err := r.client.GetCustomerWithResponse(ctx, customerID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Customer After Update",
			"Could not read customer after update: "+err.Error(),
		)
		return
	}

	if customerResp.StatusCode() != 200 || customerResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Customer After Update",
			fmt.Sprintf("Customer not found after update, status: %d, body: %s", customerResp.StatusCode(), string(customerResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(overlayCustomerComputedFields(ctx, customerResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customerResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Deleting Customer resource from Terraform state only; remote customer settings are preserved")
	resp.Diagnostics.AddWarning(
		"Customer Settings Not Deleted from DoiT API",
		"Removing the doit_customer resource from Terraform state does not delete the customer settings in DoiT. "+
			"The customer and its configuration continue to exist.",
	)
}

func (r *customerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *customerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
