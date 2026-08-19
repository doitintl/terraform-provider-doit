package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (r *customerResource) populateState(ctx context.Context, state *customerResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	customerID := ""
	if !state.Id.IsNull() && !state.Id.IsUnknown() {
		customerID = state.Id.ValueString()
	} else if !state.CustomerId.IsNull() && !state.CustomerId.IsUnknown() {
		customerID = state.CustomerId.ValueString()
	}

	if customerID == "" {
		diags.AddError(
			"Error Reading Customer",
			"Customer ID is required to read customer settings",
		)
		return diags
	}

	customerResp, err := r.client.GetCustomerWithResponse(ctx, customerID)
	if err != nil {
		diags.AddError(
			"Error Reading Customer",
			"Could not read customer, unexpected error: "+err.Error(),
		)
		return diags
	}

	if customerResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		state.CustomerId = types.StringNull()
		return diags
	}

	if customerResp.StatusCode() != 200 || customerResp.JSON200 == nil {
		diags.AddError(
			"Error Reading Customer",
			fmt.Sprintf("Could not read customer, status: %d, body: %s", customerResp.StatusCode(), string(customerResp.Body)),
		)
		return diags
	}

	diags.Append(mapCustomerToModel(ctx, customerResp.JSON200, state)...)
	return diags
}

func mapCustomerToModel(ctx context.Context, customer *models.Customer, state *customerResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if customer == nil {
		return diags
	}

	state.Id = types.StringValue(customer.Id)
	state.CustomerId = types.StringValue(customer.Id)
	state.Name = types.StringPointerValue(customer.Name)
	state.PrimaryDomain = types.StringPointerValue(customer.PrimaryDomain)

	var d diag.Diagnostics
	state.Domains, d = mapStringList(ctx, customer.Domains)
	diags.Append(d...)

	if customer.UrlSlug == nil && !state.UrlSlug.IsNull() && !state.UrlSlug.IsUnknown() && state.UrlSlug.ValueString() == "" {
		state.UrlSlug = types.StringValue("")
	} else {
		state.UrlSlug = types.StringPointerValue(customer.UrlSlug)
	}

	if customer.Settings != nil {
		allowedInviteDomainsVal, d := mapStringList(ctx, customer.Settings.AllowedInviteDomains)
		diags.Append(d...)

		var currencyVal types.String
		if customer.Settings.Currency != nil {
			currencyVal = types.StringValue(string(*customer.Settings.Currency))
		} else {
			currencyVal = types.StringNull()
		}

		var mfaRequiredVal types.Bool
		if customer.Settings.MfaRequired != nil {
			mfaRequiredVal = types.BoolValue(*customer.Settings.MfaRequired)
		} else {
			mfaRequiredVal = types.BoolNull()
		}

		settingsVal, d := resource_customer.NewSettingsValue(resource_customer.SettingsValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"allowed_invite_domains": allowedInviteDomainsVal,
			"currency":               currencyVal,
			"mfa_required":           mfaRequiredVal,
		})
		diags.Append(d...)
		state.Settings = settingsVal
	} else {
		state.Settings = resource_customer.NewSettingsValueNull()
	}

	if customer.Contact != nil {
		var emails []string
		if customer.Contact.Emails != nil {
			for _, email := range *customer.Contact.Emails {
				emails = append(emails, string(email))
			}
		}
		emailsVal, d := mapStringList(ctx, &emails)
		diags.Append(d...)

		contactVal, d := resource_customer.NewContactValue(resource_customer.ContactValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"emails": emailsVal,
		})
		diags.Append(d...)
		state.Contact = contactVal
	} else {
		isConfiguredEmpty := !state.Contact.IsNull() && !state.Contact.IsUnknown() &&
			!state.Contact.Emails.IsNull() && !state.Contact.Emails.IsUnknown() &&
			len(state.Contact.Emails.Elements()) == 0

		if !isConfiguredEmpty {
			state.Contact = resource_customer.NewContactValueNull()
		}
	}

	return diags
}

func overlayCustomerComputedFields(ctx context.Context, apiResp *models.Customer, plan *customerResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapCustomerToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay.
	plan.Id = resolved.Id
	if plan.CustomerId.IsUnknown() {
		plan.CustomerId = resolved.CustomerId
	}
	plan.Name = resolved.Name
	plan.PrimaryDomain = resolved.PrimaryDomain
	plan.Domains = resolved.Domains

	if plan.UrlSlug.IsUnknown() {
		plan.UrlSlug = resolved.UrlSlug
	}

	if plan.Settings.IsUnknown() {
		plan.Settings = resolved.Settings
	} else if !plan.Settings.IsNull() {
		overlayCustomerSettings(ctx, &resolved.Settings, &plan.Settings)
	}

	if plan.Contact.IsUnknown() {
		plan.Contact = resolved.Contact
	} else if !plan.Contact.IsNull() {
		overlayCustomerContact(ctx, &resolved.Contact, &plan.Contact)
	}

	return diags
}

func overlayCustomerSettings(_ context.Context, resolved, plan *resource_customer.SettingsValue) {
	if plan.Currency.IsUnknown() {
		plan.Currency = resolved.Currency
	}
	if plan.AllowedInviteDomains.IsUnknown() {
		plan.AllowedInviteDomains = resolved.AllowedInviteDomains
	}
	if plan.MfaRequired.IsUnknown() {
		plan.MfaRequired = resolved.MfaRequired
	}
}

func overlayCustomerContact(_ context.Context, resolved, plan *resource_customer.ContactValue) {
	if plan.Emails.IsUnknown() {
		plan.Emails = resolved.Emails
	}
}

func (plan *customerResourceModel) toUpdateRequest(ctx context.Context, config *customerResourceModel) (models.CustomerUpdate, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := models.CustomerUpdate{}

	if !plan.UrlSlug.IsNull() && !plan.UrlSlug.IsUnknown() {
		req.UrlSlug = new(plan.UrlSlug.ValueString())
	} else if config == nil || config.UrlSlug.IsNull() {
		req.UrlSlug = new("")
	}

	if !plan.Settings.IsNull() && !plan.Settings.IsUnknown() {
		hasSettings := false
		settings := models.CustomerSettings{}

		// currency: Category B — only send when explicitly configured in HCL (not when defaulted/copied from prior state)
		if config != nil && !config.Settings.IsNull() && !config.Settings.Currency.IsNull() && !config.Settings.Currency.IsUnknown() {
			if !plan.Settings.Currency.IsNull() && !plan.Settings.Currency.IsUnknown() {
				settings.Currency = new(models.Currency(plan.Settings.Currency.ValueString()))
				hasSettings = true
			}
		}

		// allowed_invite_domains: Category A — send planned list (including empty slice when cleared)
		if !plan.Settings.AllowedInviteDomains.IsNull() && !plan.Settings.AllowedInviteDomains.IsUnknown() {
			var domains []string
			diags.Append(plan.Settings.AllowedInviteDomains.ElementsAs(ctx, &domains, false)...)
			if domains == nil {
				domains = []string{}
			}
			settings.AllowedInviteDomains = &domains
			hasSettings = true
		} else if config == nil || config.Settings.IsNull() || config.Settings.AllowedInviteDomains.IsNull() {
			domains := []string{}
			settings.AllowedInviteDomains = &domains
			hasSettings = true
		}

		// mfa_required: Category B — only send when explicitly configured in HCL (not when defaulted/copied from prior state)
		if config != nil && !config.Settings.IsNull() && !config.Settings.MfaRequired.IsNull() && !config.Settings.MfaRequired.IsUnknown() {
			if !plan.Settings.MfaRequired.IsNull() && !plan.Settings.MfaRequired.IsUnknown() {
				settings.MfaRequired = plan.Settings.MfaRequired.ValueBoolPointer()
				hasSettings = true
			}
		}

		if hasSettings {
			req.Settings = &settings
		}
	} else if config == nil || config.Settings.IsNull() {
		// settings block omitted in HCL: clear Category A (allowed_invite_domains) while preserving Category B
		domains := []string{}
		req.Settings = &models.CustomerSettings{
			AllowedInviteDomains: &domains,
		}
	}

	if !plan.Contact.IsNull() && !plan.Contact.IsUnknown() {
		// emails: Category A — send planned list (including empty slice when cleared)
		if !plan.Contact.Emails.IsNull() && !plan.Contact.Emails.IsUnknown() {
			var emails []string
			diags.Append(plan.Contact.Emails.ElementsAs(ctx, &emails, false)...)
			openapiEmails := make([]openapi_types.Email, 0, len(emails))
			for _, email := range emails {
				openapiEmails = append(openapiEmails, openapi_types.Email(email))
			}
			req.Contact = &models.CustomerContact{
				Emails: &openapiEmails,
			}
		} else {
			emptyEmails := []openapi_types.Email{}
			req.Contact = &models.CustomerContact{
				Emails: &emptyEmails,
			}
		}
	} else if config == nil || config.Contact.IsNull() {
		// contact block omitted in HCL: clear Category A (emails)
		emptyEmails := []openapi_types.Email{}
		req.Contact = &models.CustomerContact{
			Emails: &emptyEmails,
		}
	}

	return req, diags
}
