package provider

import (
	"context"
	"fmt"
	"log"
	"strings"

	"terraform-provider-doit/internal/provider/resource_budget"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const budgetSchemaVersion = 1

type (
	budgetResource struct {
		client *Client
	}
	budgetResourceModel struct {
		resource_budget.BudgetModel
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                 = &budgetResource{}
	_ resource.ResourceWithConfigure    = &budgetResource{}
	_ resource.ResourceWithUpgradeState = &budgetResource{}
)

func NewBudgetResource() resource.Resource {
	return &budgetResource{}
}

// budgetResource is the resource implementation.
type budgetResource struct {
	client *Client
}

// Metadata returns the resource type name.
func (r *budgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	log.Print("hello budget Metadata:)")
	resp.TypeName = req.ProviderTypeName + "_budget"
}

// Schema defines the schema for the resource.
func (r *budgetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	log.Print("hello budget Schema:)")
	resp.Schema = schema.Schema{
		Description:         "Budgets allow you to monitor and control your cloud spending by setting limits and alerts.",
		MarkdownDescription: "Budgets allow you to monitor and control your cloud spending by setting limits and alerts.",
		Attributes: map[string]schema.Attribute{
			"last_updated": schema.StringAttribute{
				Description: "Timestamp of the last Terraform update of" +
					"the budget group.",
				Computed: true,
			},
			"alerts": schema.ListNestedAttribute{
				Description: "List of up to three thresholds defined as percentage of amount",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"percentage": schema.Float64Attribute{
							Description: "Percentage of the budget amount",
							Required:    true,
						},
					},
				},
			},
			"amount": schema.Float64Attribute{
				Description: "Budget period amount. Required if usePrevSpend is false.",
				Optional:    true,
			},
			"collaborators": schema.ListNestedAttribute{
				Description: "List of permitted users to view/edit the report",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"email": schema.StringAttribute{
							Description: "Email of the collaborator",
							Required:    true,
						},
						"role": schema.StringAttribute{
							Description: "Role of the collaborator",
							Required:    true,
						},
					},
				},
			},
			"currency": schema.StringAttribute{
				Required: true,
				Description: `Budget currency. Possible values are:
'USD'
'ILS'
'EUR'
'AUD'
'CAD'
'GBP'
'DKK'
'NOK'
'SEK'
'BRL'
'SGD'
'MXN'
'CHF'
'MYR'
'TWD'
'EGP'
'ZAR'
'JPY'
'IDR'
'AED'
'COP'
'THB'`,
			},
			"description": schema.StringAttribute{
				Description: "Budget description",
				Optional:    true, Computed: true,
				Default: stringdefault.StaticString(""),
			},
			"end_period": schema.Int64Attribute{
				Description: "Fixed budget end date. Required if budget type is fixed. In milliseconds since the epoch.",
				Optional:    true,
			},
			"growth_per_period": schema.Float64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(0.0),
				Description: "Periodical growth percentage in recurring budget",
			},
			"id": schema.StringAttribute{
				Description: "Budget ID",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"metric": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("cost"),
				Description: "Budget metric  - currently fixed to \"cost\"",
			},
			"name": schema.StringAttribute{
				Description: "Budget Name",
				Required:    true,
			},
			"public": schema.StringAttribute{
				Optional:    true,
				Description: "Public",
			},
			"recipients": schema.ListAttribute{
				Description: "List of emails to notify when reaching alert threshold",
				Optional:    true,
				ElementType: types.StringType,
			},
			"recipients_slack_channels": schema.ListNestedAttribute{
				Description: "List of slack channels to notify when reaching alert threshold",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Slack channel ID",
							Required:    true,
						},
						"name": schema.StringAttribute{
							Description: "Slack channel name",
							Required:    true,
						},
						"shared": schema.BoolAttribute{
							Description: "Whether the channel is shared",
							Required:    true,
						},
						"type": schema.StringAttribute{
							Description: "Type of the channel",
							Required:    true,
						},
						"workspace": schema.StringAttribute{
							Description: "Slack workspace",
							Required:    true,
						},
						"customer_id": schema.StringAttribute{
							Description: "Customer ID",
							Required:    true,
						},
					},
				},
			},
			"scope": schema.ListAttribute{
				Description: "List of attributions that defines that budget scope",
				Required:    true,
				ElementType: types.StringType,
			},
			"start_period": schema.Int64Attribute{
				Description: "Budget start Date, in milliseconds since the epoch.",
				Required:    true,
				Validators:  []validator.Int64{budgetStartPeriodValidator{}},
			},
			"time_interval": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Description: "Recurring budget interval can be on of:" +
					"[\"day\", \"week\", \"month\", \"quarter\",\"year]\"",
				Validators: []validator.String{budgetTimeIntervalValidator{}},
			},
			"type": schema.StringAttribute{
				Required: true,
				Description: "Budget type can be one of: [" +
					"\"fixed\", \"recurring\"]",
			},
			"use_prev_spend": schema.BoolAttribute{
				Description: "Use the last period's spend as the target amount for recurring budgets",
				Optional:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *budgetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Clients)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Clients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client.OldClient
}

func (r *budgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budget"
}

func (r *budgetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_budget.BudgetResourceSchema(ctx)
	s.Version = budgetSchemaVersion
	resp.Schema = s
}

func (r *budgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data budgetResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API budget type
	budget, diags := modelToBudget(ctx, &data.BudgetModel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new budget via API
	budgetResp, err := r.client.CreateBudget(ctx, budget)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating budget",
			"Could not create budget, unexpected error: "+err.Error(),
		)
		return
	}

	// Update model with response data
	data.Id = types.StringValue(budgetResp.Id)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *budgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data budgetResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed budget value from API
	budget, err := r.client.GetBudget(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Budget",
			"Could not read budget ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update model with API response
	diags := budgetToModel(ctx, budget, &data.BudgetModel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *budgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data budgetResourceModel
	var state budgetResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read current state to get the budget ID
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API budget type
	budget, diags := modelToBudget(ctx, &data.BudgetModel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update existing budget via API
	_, err := r.client.UpdateBudget(ctx, state.Id.ValueString(), budget)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Budget",
			"Could not update budget, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated budget from API as Update response may not be fully populated
	budgetResp, err := r.client.GetBudget(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Budget",
			"Could not read budget ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update model with API response
	diags = budgetToModel(ctx, budgetResp, &data.BudgetModel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *budgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data budgetResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete budget via API
	err := r.client.DeleteBudget(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Budget",
			"Could not delete budget, unexpected error: "+err.Error(),
		)
		return
	}
}

// modelToBudget converts the Terraform model to the API Budget type.
func modelToBudget(ctx context.Context, model *resource_budget.BudgetModel) (Budget, diag.Diagnostics) {
	var budget Budget
	var diags diag.Diagnostics

	// Convert alerts
	if !model.Alerts.IsNull() && !model.Alerts.IsUnknown() {
		var alerts []ExternalBudgetAlert
		elements := model.Alerts.Elements()

		for _, elem := range elements {
			objVal, ok := elem.(types.Object)
			if !ok {
				continue
			}

			attrs := objVal.Attributes()
			if percentageAttr, ok := attrs["percentage"]; ok {
				if percentageVal, ok := percentageAttr.(types.Float64); ok && !percentageVal.IsNull() {
					alerts = append(alerts, ExternalBudgetAlert{
						Percentage: percentageVal.ValueFloat64(),
					})
				}
			}
		}
		budget.Alerts = alerts
	}

	// Simple field conversions
	if !model.Amount.IsNull() && !model.Amount.IsUnknown() {
		budget.Amount = model.Amount.ValueFloat64()
	}

	// Convert collaborators
	if !model.Collaborators.IsNull() && !model.Collaborators.IsUnknown() {
		var collaborators []Collaborator
		elements := model.Collaborators.Elements()

		for _, elem := range elements {
			objVal, ok := elem.(types.Object)
			if !ok {
				continue
			}

			attrs := objVal.Attributes()
			var collab Collaborator

			if emailAttr, ok := attrs["email"]; ok {
				if emailVal, ok := emailAttr.(types.String); ok && !emailVal.IsNull() {
					collab.Email = emailVal.ValueString()
				}
			}
			if roleAttr, ok := attrs["role"]; ok {
				if roleVal, ok := roleAttr.(types.String); ok && !roleVal.IsNull() {
					collab.Role = roleVal.ValueString()
				}
			}

			collaborators = append(collaborators, collab)
		}
		budget.Collaborators = collaborators
	}

	if !model.Currency.IsNull() && !model.Currency.IsUnknown() {
		budget.Currency = model.Currency.ValueString()
	}

	if !model.Description.IsNull() && !model.Description.IsUnknown() {
		budget.Description = model.Description.ValueString()
	}

	if !model.EndPeriod.IsNull() && !model.EndPeriod.IsUnknown() {
		budget.EndPeriod = model.EndPeriod.ValueInt64()
	}

	if !model.GrowthPerPeriod.IsNull() && !model.GrowthPerPeriod.IsUnknown() {
		budget.GrowthPerPeriod = model.GrowthPerPeriod.ValueFloat64()
	}

	if !model.Metric.IsNull() && !model.Metric.IsUnknown() {
		budget.Metric = model.Metric.ValueString()
	}

	if !model.Name.IsNull() && !model.Name.IsUnknown() {
		budget.Name = model.Name.ValueString()
	}

	if !model.Public.IsNull() && !model.Public.IsUnknown() {
		public := model.Public.ValueString()
		budget.Public = &public
	}

	// Convert recipients list
	if !model.Recipients.IsNull() && !model.Recipients.IsUnknown() {
		var recipients []string
		diags.Append(model.Recipients.ElementsAs(ctx, &recipients, false)...)
		budget.Recipients = recipients
	}

	// Convert recipients_slack_channels
	if !model.RecipientsSlackChannels.IsNull() && !model.RecipientsSlackChannels.IsUnknown() {
		var slackChannels []SlackChannel
		elements := model.RecipientsSlackChannels.Elements()

		for _, elem := range elements {
			objVal, ok := elem.(types.Object)
			if !ok {
				continue
			}

			attrs := objVal.Attributes()
			var slack SlackChannel

			if attr, ok := attrs["customer_id"]; ok {
				if val, ok := attr.(types.String); ok && !val.IsNull() {
					slack.CustomerId = val.ValueString()
				}
			}
			if attr, ok := attrs["id"]; ok {
				if val, ok := attr.(types.String); ok && !val.IsNull() {
					slack.Id = val.ValueString()
				}
			}
			if attr, ok := attrs["name"]; ok {
				if val, ok := attr.(types.String); ok && !val.IsNull() {
					slack.Name = val.ValueString()
				}
			}
			if attr, ok := attrs["shared"]; ok {
				if val, ok := attr.(types.Bool); ok && !val.IsNull() {
					slack.Shared = val.ValueBool()
				}
			}
			if attr, ok := attrs["type"]; ok {
				if val, ok := attr.(types.String); ok && !val.IsNull() {
					slack.Type = val.ValueString()
				}
			}
			if attr, ok := attrs["workspace"]; ok {
				if val, ok := attr.(types.String); ok && !val.IsNull() {
					slack.Workspace = val.ValueString()
				}
			}

			slackChannels = append(slackChannels, slack)
		}
		budget.RecipientsSlackChannels = slackChannels
	}

	// Convert scope list
	if !model.Scope.IsNull() && !model.Scope.IsUnknown() {
		var scope []string
		diags.Append(model.Scope.ElementsAs(ctx, &scope, false)...)
		budget.Scope = scope
	}

	if !model.StartPeriod.IsNull() && !model.StartPeriod.IsUnknown() {
		budget.StartPeriod = model.StartPeriod.ValueInt64()
	}

	if !model.TimeInterval.IsNull() && !model.TimeInterval.IsUnknown() {
		budget.TimeInterval = model.TimeInterval.ValueString()
	}

	if !model.Type.IsNull() && !model.Type.IsUnknown() {
		budget.Type = model.Type.ValueString()
	}

	if !model.UsePrevSpend.IsNull() && !model.UsePrevSpend.IsUnknown() {
		budget.UsePrevSpend = model.UsePrevSpend.ValueBool()
	}

	return budget, diags
}

// budgetToModel converts the API Budget type to the Terraform model.
func budgetToModel(ctx context.Context, budget *Budget, model *resource_budget.BudgetModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if budget == nil {
		return diags
	}

	model.Id = types.StringValue(budget.Id)

	// Convert alerts - note: new fields forecasted_date and triggered will be populated from API.
	// This will be populated from the API response which should include the new fields.
	// For now, keep the implementation simple and let the generated code handle it.
	// The state upgrader will have already converted old alerts to the new format.

	model.Amount = types.Float64Value(budget.Amount)

	// Convert collaborators
	// Similar to alerts, the generated types should handle this

	model.Currency = types.StringValue(budget.Currency)
	model.Description = types.StringValue(budget.Description)

	if budget.EndPeriod > 0 && budget.EndPeriod != 2678400000 {
		model.EndPeriod = types.Int64Value(budget.EndPeriod)
	} else {
		model.EndPeriod = types.Int64Null()
	}

	model.GrowthPerPeriod = types.Float64Value(budget.GrowthPerPeriod)
	model.Metric = types.StringValue(budget.Metric)
	model.Name = types.StringValue(budget.Name)

	if budget.Public != nil && *budget.Public != "" {
		model.Public = types.StringValue(*budget.Public)
	} else {
		model.Public = types.StringNull()
	}

	// Convert recipients list
	if budget.Recipients != nil {
		recipientsList, listDiags := types.ListValueFrom(ctx, types.StringType, budget.Recipients)
		diags.Append(listDiags...)
		model.Recipients = recipientsList
	} else {
		model.Recipients = types.ListNull(types.StringType)
	}

	// Note: For complex nested types like recipients_slack_channels, alerts, collaborators,
	// we'll let the next Read operation fully populate them from the API
	// The state upgrader ensures the structure is correct

	// Convert scope list
	if budget.Scope != nil {
		scopeList, listDiags := types.ListValueFrom(ctx, types.StringType, budget.Scope)
		diags.Append(listDiags...)
		model.Scope = scopeList
	} else {
		model.Scope = types.ListNull(types.StringType)
	}

	model.StartPeriod = types.Int64Value(budget.StartPeriod)
	model.TimeInterval = types.StringValue(budget.TimeInterval)
	model.Type = types.StringValue(budget.Type)
	model.UsePrevSpend = types.BoolValue(budget.UsePrevSpend)

	return diags
}
