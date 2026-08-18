package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customerDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customerDataSource)(nil)

func NewCustomerDataSource() datasource.DataSource {
	return &customerDataSource{}
}

type customerDataSource struct {
	client *models.ClientWithResponses
}

type customerDataSourceModel struct {
	datasource_customer.CustomerModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *customerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer"
}

func (d *customerDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_customer.CustomerDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *customerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *customerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customerDataSourceModel

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

	// No inputs to check for unknown — this is a singleton endpoint with no parameters.

	customerResp, err := d.client.GetCustomerWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Customer",
			"Could not read customer, unexpected error: "+err.Error(),
		)
		return
	}

	if customerResp.StatusCode() != 200 || customerResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Customer",
			fmt.Sprintf("Could not read customer, status: %d, body: %s", customerResp.StatusCode(), string(customerResp.Body)),
		)
		return
	}

	customer := customerResp.JSON200

	data.Id = types.StringValue(customer.Id)
	data.Name = types.StringPointerValue(customer.Name)
	data.PrimaryDomain = types.StringPointerValue(customer.PrimaryDomain)

	if customer.Domains != nil && len(*customer.Domains) > 0 {
		var d diag.Diagnostics
		data.Domains, d = types.ListValueFrom(ctx, types.StringType, *customer.Domains)
		resp.Diagnostics.Append(d...)
	} else {
		var d diag.Diagnostics
		data.Domains, d = types.ListValueFrom(ctx, types.StringType, []string{})
		resp.Diagnostics.Append(d...)
	}

	data.UrlSlug = types.StringPointerValue(customer.UrlSlug)

	if customer.Settings != nil {
		settingsAttrTypes := datasource_customer.SettingsValue{}.AttributeTypes(ctx)
		var allowedInviteDomainsVal types.List
		if customer.Settings.AllowedInviteDomains != nil && len(*customer.Settings.AllowedInviteDomains) > 0 {
			var d diag.Diagnostics
			allowedInviteDomainsVal, d = types.ListValueFrom(ctx, types.StringType, *customer.Settings.AllowedInviteDomains)
			resp.Diagnostics.Append(d...)
		} else {
			var d diag.Diagnostics
			allowedInviteDomainsVal, d = types.ListValueFrom(ctx, types.StringType, []string{})
			resp.Diagnostics.Append(d...)
		}

		var currencyVal types.String
		if customer.Settings.Currency != nil {
			currencyVal = types.StringValue(string(*customer.Settings.Currency))
		} else {
			currencyVal = types.StringNull()
		}

		settingsVal, d := datasource_customer.NewSettingsValue(settingsAttrTypes, map[string]attr.Value{
			"allowed_invite_domains": allowedInviteDomainsVal,
			"currency":               currencyVal,
		})
		resp.Diagnostics.Append(d...)
		data.Settings = settingsVal
	} else {
		data.Settings = datasource_customer.NewSettingsValueNull()
	}

	if customer.Contact != nil {
		contactAttrTypes := datasource_customer.ContactValue{}.AttributeTypes(ctx)
		var emailsVal types.List
		if customer.Contact.Emails != nil && len(*customer.Contact.Emails) > 0 {
			var emails []string
			for _, email := range *customer.Contact.Emails {
				emails = append(emails, string(email))
			}
			var d diag.Diagnostics
			emailsVal, d = types.ListValueFrom(ctx, types.StringType, emails)
			resp.Diagnostics.Append(d...)
		} else {
			var d diag.Diagnostics
			emailsVal, d = types.ListValueFrom(ctx, types.StringType, []string{})
			resp.Diagnostics.Append(d...)
		}

		contactVal, d := datasource_customer.NewContactValue(contactAttrTypes, map[string]attr.Value{
			"emails": emailsVal,
		})
		resp.Diagnostics.Append(d...)
		data.Contact = contactVal
	} else {
		data.Contact = datasource_customer.NewContactValueNull()
	}

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
