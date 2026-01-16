package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &doitProvider{}
)

// HostURL is the default DoiT API URL.
const HostURL string = "https://api.doit.com"

// doitProviderModel maps provider schema data to a Go type.
type doitProviderModel struct {
	Host            types.String `tfsdk:"host"`
	DoiTAPITOken    types.String `tfsdk:"api_token"`
	CustomerContext types.String `tfsdk:"customer_context"`
}

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &doitProvider{
			version: version,
		}
	}
}

// doitProvider is the provider implementation.
type doitProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *doitProvider) Metadata(ctx context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	tflog.Debug(ctx, "provider Metadata")
	resp.TypeName = "doit"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *doitProvider) Schema(ctx context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	tflog.Debug(ctx, "provider Schema")
	resp.Schema = schema.Schema{
		Description: "DoiT Cloud Intelligence Terraform Provider. Note that this version of the provider is still currently in a preview state and it is not currently supported.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "URI for DoiT API. May also be provided via DOIT_HOST environment variable.",
				Optional:    true,
			},
			"api_token": schema.StringAttribute{
				Description: "API Token to access DoiT API. May also be provided by DOIT_API_TOKEN " +
					"environment variable. Refer to " +
					"https://developer.doit.com/docs/start",
				Optional:  true,
				Sensitive: true,
			},
			"customer_context": schema.StringAttribute{
				Description: "Customer context. May also be provided by DOIT_CUSTOMER_CONTEXT " +
					"environment variable. This field is requiered just for DoiT employees ",
				Optional: true,
			},
		},
	}
}

// Configure prepares a doit API client for data sources and resources.
func (p *doitProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Debug(ctx, "provider Configure")

	doiTAPIToken := ""
	host := ""
	customerContext := ""
	tflog.Info(ctx, "Configuring DoiT client")
	tflog.Trace(ctx, "[TRACE] Calling Program::")

	ctx = tflog.SetField(ctx, "doit_host", host)
	ctx = tflog.SetField(ctx, "doit_api_token", doiTAPIToken)
	ctx = tflog.SetField(ctx, "doit_customer_context", customerContext)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "doit_api_token")

	tflog.Debug(ctx, "Creating DoiT Console client")

	// Retrieve provider data from configuration
	var config doitProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown DoiT API Host",
			"-The provider cannot create the DoiT API client as there is an unknown configuration value for the DoiT API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the DOIT_HOST environment variable.",
		)
	}

	if config.DoiTAPITOken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("doiTAPITOken"),
			"Unknown DoiT API DoiTAPITOken",
			"-The provider cannot create the DoiT API client as there is an unknown configuration value for the DoiT API doiTAPITOken. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the DOIT_API_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.
	doiTAPIToken = os.Getenv("DOIT_API_TOKEN")
	host = os.Getenv("DOIT_HOST")
	customerContext = os.Getenv("DOIT_CUSTOMER_CONTEXT")

	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}

	if !config.CustomerContext.IsNull() {
		customerContext = config.CustomerContext.ValueString()
	}

	if !config.DoiTAPITOken.IsNull() {
		doiTAPIToken = config.DoiTAPITOken.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if host == "" {
		/*resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing DoiT API Host",
			"The provider cannot create the DoiT API client as there is a missing or empty value for the DoiT API host. "+
				"Set the host value in the configuration or use the DOIT_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)*/
		host = HostURL
	}

	if doiTAPIToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("doiTAPIToken"),
			"Missing DoiT API Token",
			"The provider cannot create the DoiT API client as there is a missing or empty value for the DoiT API token. "+
				"Set the doiTAPIToken value in the configuration or use the DOIT_API_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create a new DoiT client using the configuration values
	client, err := NewClient(ctx, host, doiTAPIToken, customerContext)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create DoiT API Client",
			"An unexpected error occurred when creating the DoiT API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"DoiT Client Error: "+err.Error(),
		)
		return
	}

	// Make the DoiT client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured DoiT client", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *doitProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// Resources defines the resources implemented in the provider.
func (p *doitProvider) Resources(ctx context.Context) []func() resource.Resource {
	tflog.Debug(ctx, "provider Resources")
	return []func() resource.Resource{
		NewReportResource,
		NewBudgetResource,
		NewAllocationResource,
		NewLabelResource,
	}
}
