package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// orderResourceModel maps the resource schema data.
type budgetResourceModel struct {
	// Alerts List of up to three thresholds defined as percentage of amount
	Alerts []ExternalBudgetAlertModel `tfsdk:"alerts"`

	// Amount Budget period amount
	// Optional: true(if usePrevSpend is false)
	Amount types.Float64 `tfsdk:"amount"`

	// Collaborators List of permitted users to view/edit the report
	Collaborators []CollaboratorModel `tfsdk:"collaborators"`

	// Currency Budget currency can be one of: ["USD","ILS","EUR","GBP","AUD","CAD","DKK","NOK","SEK","BRL","SGD","MXN","CHF","MYR","TWD"]
	Currency types.String `tfsdk:"currency"`

	// Description Budget description
	Description types.String `tfsdk:"description"`

	// EndPeriod Fixed budget end date
	// Optional: true(if budget type is fixed)
	EndPeriod types.Int64 `tfsdk:"end_period"`

	// GrowthPerPeriod Periodical growth percentage in recurring budget
	GrowthPerPeriod types.Float64 `tfsdk:"growth_per_period"`

	// Id budget ID, identifying the report
	// in:path
	Id types.String `tfsdk:"id"`

	// Metric Budget metric - currently fixed to "cost"
	Metric types.String `tfsdk:"metric"`

	// Name Budget Name
	Name   types.String `tfsdk:"name"`
	Public types.String `tfsdk:"public"`

	// Recipients List of emails to notify when reaching alert threshold
	Recipients []types.String `tfsdk:"recipients"`

	// RecipientsSlackChannels List of slack channels to notify when reaching alert threshold
	RecipientsSlackChannels []SlackChannelModel `tfsdk:"recipients_slack_channels"`

	// Scope List of budges that defines that budget scope
	Scope []types.String `tfsdk:"scope"`

	// StartPeriod Budget start Date
	StartPeriod types.Int64 `tfsdk:"start_period"`

	// TimeInterval Recurring budget interval can be on of: ["day", "week", "month", "quarter","year"]
	TimeInterval types.String `tfsdk:"time_interval"`

	// Type budget type can be one of: ["fixed", "recurring"]
	Type types.String `tfsdk:"type"`

	// UsePrevSpend Use the last period's spend as the target amount for recurring budgets
	UsePrevSpend types.Bool `tfsdk:"use_prev_spend"`

	LastUpdated types.String `tfsdk:"last_updated"`
}

type ExternalBudgetAlertModel struct {
	Percentage types.Float64 `tfsdk:"percentage"`
}

type CollaboratorModel struct {
	Email types.String `tfsdk:"email"`
	Role  types.String `tfsdk:"role"`
}

// SlackChannel defines model for SlackChannel.
type SlackChannelModel struct {
	CustomerId types.String `tfsdk:"customer_id"`
	Id         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Shared     types.Bool   `tfsdk:"shared"`
	Type       types.String `tfsdk:"type"`
	Workspace  types.String `tfsdk:"workspace"`
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &budgetResource{}
	_ resource.ResourceWithConfigure = &budgetResource{}
)

// NewbudgetResource is a helper function to simplify the provider implementation.
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
	log.Print(" budget Configure")
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

func budgetModelToBudget(budgetModel *budgetResourceModel) Budget {
	var budget Budget
	var alerts []ExternalBudgetAlert
	for _, alert := range budgetModel.Alerts {
		percentage := alert.Percentage.ValueFloat64()
		alerts = append(alerts, ExternalBudgetAlert{
			Percentage: percentage,
		})
	}
	budget.Alerts = alerts
	amount := budgetModel.Amount.ValueFloat64()
	budget.Amount = amount
	var collaborators []Collaborator
	for _, collaborator := range budgetModel.Collaborators {
		email := collaborator.Email.ValueString()
		role := collaborator.Role.ValueString()
		collaborators = append(collaborators, Collaborator{
			Email: email,
			Role:  role,
		})
	}
	budget.Collaborators = collaborators
	budget.Currency = budgetModel.Currency.ValueString()
	description := budgetModel.Description.ValueString()
	budget.Description = description
	endPeriod := budgetModel.EndPeriod.ValueInt64()
	budget.EndPeriod = endPeriod
	growthPerPeriod := budgetModel.GrowthPerPeriod.ValueFloat64()
	budget.GrowthPerPeriod = growthPerPeriod
	metric := budgetModel.Metric.ValueString()
	budget.Metric = metric
	budget.Name = budgetModel.Name.ValueString()
	public := budgetModel.Public.ValueString()
	budget.Public = &public
	var recipients []string
	for _, recipient := range budgetModel.Recipients {
		recipients = append(recipients, recipient.ValueString())
	}
	budget.Recipients = recipients

	var slackChannels []SlackChannel
	if budgetModel.RecipientsSlackChannels != nil {
		for _, slackChannel := range budgetModel.RecipientsSlackChannels {
			customerId := slackChannel.CustomerId.ValueString()
			id := slackChannel.Id.ValueString()
			name := slackChannel.Name.ValueString()
			shared := slackChannel.Shared.ValueBool()
			typee := slackChannel.Type.ValueString()
			workspace := slackChannel.Workspace.ValueString()
			slackChannels = append(slackChannels, SlackChannel{
				CustomerId: customerId,
				Id:         id,
				Name:       name,
				Shared:     shared,
				Type:       typee,
				Workspace:  workspace,
			})
		}
		budget.RecipientsSlackChannels = slackChannels
	}
	var scope []string
	for _, scopee := range budgetModel.Scope {
		scope = append(scope, scopee.ValueString())
	}
	budget.Scope = scope
	budget.StartPeriod = budgetModel.StartPeriod.ValueInt64()
	budget.TimeInterval = budgetModel.TimeInterval.ValueString()
	budget.Type = budgetModel.Type.ValueString()
	usePrevSpend := budgetModel.UsePrevSpend.ValueBool()
	budget.UsePrevSpend = usePrevSpend
	return budget
}

// Create creates the resource and sets the initial Terraform state.
func (r *budgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	log.Println(" budget Create")
	log.Println(r.client.Auth.DoiTAPITOken)
	log.Println("---------------------------------------------------")
	log.Println(r.client.Auth.CustomerContext)

	// Retrieve values from plan
	var plan budgetResourceModel
	log.Println("before getting plan")
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	log.Println("after getting plan")
	// Generate API request body from plan

	budget := budgetModelToBudget(&plan)

	log.Println("before creating budget")
	// Create new budget
	budgeResponse, err := r.client.CreateBudget(ctx, budget)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating budget",
			"Could not create budget, unexpected error: "+err.Error(),
		)
		return
	}
	log.Println("budget response---------------------------------------------------")
	log.Println(budgeResponse)
	log.Println("budget id---------------------------------------------------")
	log.Println(budgeResponse.Id)
	plan.Id = types.StringValue(budgeResponse.Id)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

func budgetToBudgetResourceModel(budget *Budget, budgetModel *budgetResourceModel, _ context.Context) {
	if budget != nil {
		budgetModel.Id = types.StringValue(budget.Id)
		budgetModel.Alerts = []ExternalBudgetAlertModel{}
		for _, alert := range budget.Alerts {
			budgetModel.Alerts = append(budgetModel.Alerts, ExternalBudgetAlertModel{
				Percentage: types.Float64Value(alert.Percentage),
			})
		}
		budgetModel.Amount = types.Float64Value(budget.Amount)
		budgetModel.Collaborators = []CollaboratorModel{}
		for _, collaborator := range budget.Collaborators {
			budgetModel.Collaborators = append(budgetModel.Collaborators, CollaboratorModel{
				Email: types.StringValue(collaborator.Email),
				Role:  types.StringValue(collaborator.Role),
			})
		}
		budgetModel.Currency = types.StringValue(budget.Currency)
		budgetModel.Description = types.StringValue(budget.Description)
		if budget.EndPeriod > 0 && budget.EndPeriod != 2678400000 {
			budgetModel.EndPeriod = types.Int64Value(budget.EndPeriod)
		}
		budgetModel.GrowthPerPeriod = types.Float64Value(budget.GrowthPerPeriod)
		budgetModel.Metric = types.StringValue(budget.Metric)
		budgetModel.Type = types.StringValue(budget.Type)
		budgetModel.Name = types.StringValue(budget.Name)
		if budget.Public != nil {
			public := budget.Public
			if *public != "" {
				budgetModel.Public = types.StringValue(*public)
			}
		}
		budgetModel.Recipients = []types.String{}
		for _, recipient := range budget.Recipients {
			budgetModel.Recipients = append(budgetModel.Recipients, types.StringValue(recipient))
		}
		budgetModel.RecipientsSlackChannels = []SlackChannelModel{}
		if budget.RecipientsSlackChannels != nil {
			for _, recipient := range budget.RecipientsSlackChannels {
				budgetModel.RecipientsSlackChannels = append(budgetModel.RecipientsSlackChannels, SlackChannelModel{
					CustomerId: types.StringValue(recipient.CustomerId),
					Id:         types.StringValue(recipient.Id),
					Name:       types.StringValue(recipient.Name),
					Shared:     types.BoolValue(recipient.Shared),
					Type:       types.StringValue(recipient.Type),
					Workspace:  types.StringValue(recipient.Workspace),
				})
			}
		}
		budgetModel.Scope = []types.String{}
		for _, scope := range budget.Scope {
			budgetModel.Scope = append(budgetModel.Scope, types.StringValue(scope))
		}
		budgetModel.StartPeriod = types.Int64Value(budget.StartPeriod)
		budgetModel.TimeInterval = types.StringValue(budget.TimeInterval)
		budgetModel.Type = types.StringValue(budget.Type)
		budgetModel.UsePrevSpend = types.BoolValue(budget.UsePrevSpend)
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *budgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	log.Print(" budget Read")
	// Get current state

	var state budgetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Print("state id::::::::::::::::::::::::::)")
	log.Print(state.Id.ValueString())
	// Get refreshed budget value from DoiT
	budget, err := r.client.GetBudget(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Budget",
			"Could not read Doit Console Budget ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	budgetToBudgetResourceModel(budget, &state, ctx)
	log.Print("response::::::::::::::::::::::::::)")
	log.Print(budget)
	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	log.Print("state read::::::::::::::::::::::::::)")
	log.Print(state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *budgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	log.Print("hello budget Update:)")
	// Retrieve values from plan
	var plan budgetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state budgetResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	budget := budgetModelToBudget(&plan)

	// Update existing budget
	_, err := r.client.UpdateBudget(ctx, state.Id.ValueString(), budget)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Budget",
			"Could not update budget, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated items from GetBudget as UpdateBudget items are not
	// populated.
	budgetResponse, err := r.client.GetBudget(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Budget",
			"Could not read budget ID "+plan.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	budgetToBudgetResourceModel(budgetResponse, &plan, ctx)

	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.

func (r *budgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	log.Print("hello budget Delete:)")
	// Retrieve values from state
	var state budgetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing budge
	err := r.client.DeleteBudget(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Budget",
			"Could not delete budget, unexpected error: "+err.Error(),
		)
		return
	}
}
