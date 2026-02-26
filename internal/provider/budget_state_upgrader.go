package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// UpgradeState implements the ResourceWithUpgradeState interface.
// This method handles migrating state from schema v0 (old budget_resource.go)
// to schema v1 (new budget_resource_new.go with auto-generated schema).
func (r *budgetResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		// State upgrade from v0 (old schema) to v1 (new schema)
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"alerts": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"percentage": schema.Float64Attribute{
									Optional: true,
									Computed: true,
								},
							},
						},
						Optional: true,
					},
					"amount": schema.Float64Attribute{
						Optional: true,
						Computed: true,
					},
					"collaborators": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"email": schema.StringAttribute{
									Required: true,
								},
								"role": schema.StringAttribute{
									Required: true,
								},
							},
						},
						Required: true,
					},
					"currency": schema.StringAttribute{
						Required: true,
					},
					"description": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"end_period": schema.Int64Attribute{
						Optional: true,
					},
					"growth_per_period": schema.Float64Attribute{
						Optional: true,
						Computed: true,
					},
					"id": schema.StringAttribute{
						Computed: true,
					},
					"metric": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"public": schema.StringAttribute{
						Optional: true,
					},
					"recipients": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
					},
					"recipients_slack_channels": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"customer_id": schema.StringAttribute{
									Optional: true,
								},
								"id": schema.StringAttribute{
									Optional: true,
								},
								"name": schema.StringAttribute{
									Optional: true,
								},
								"shared": schema.BoolAttribute{
									Optional: true,
								},
								"type": schema.StringAttribute{
									Optional: true,
								},
								"workspace": schema.StringAttribute{
									Optional: true,
								},
							},
						},
						Optional: true,
					},
					"scope": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
					},
					"start_period": schema.Int64Attribute{
						Optional: true,
						Computed: true,
					},
					"time_interval": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"type": schema.StringAttribute{
						Required: true,
					},
					"use_prev_spend": schema.BoolAttribute{
						Optional: true,
						Computed: true,
					},
					"last_updated": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			StateUpgrader: upgradeBudgetStateV0ToV1,
		},
	}
}

// upgradeBudgetStateV0ToV1 handles the actual state transformation.
func upgradeBudgetStateV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	// Define the old state structure
	type oldAlertModel struct {
		Percentage types.Float64 `tfsdk:"percentage"`
	}

	type oldCollaboratorModel struct {
		Email types.String `tfsdk:"email"`
		Role  types.String `tfsdk:"role"`
	}

	type oldSlackChannelModel struct {
		CustomerId types.String `tfsdk:"customer_id"` //nolint:revive // Field name must match old state schema
		Id         types.String `tfsdk:"id"`          //nolint:revive // Field name must match old state schema
		Name       types.String `tfsdk:"name"`
		Shared     types.Bool   `tfsdk:"shared"`
		Type       types.String `tfsdk:"type"`
		Workspace  types.String `tfsdk:"workspace"`
	}

	type oldBudgetState struct {
		Alerts                  []oldAlertModel        `tfsdk:"alerts"`
		Amount                  types.Float64          `tfsdk:"amount"`
		Collaborators           []oldCollaboratorModel `tfsdk:"collaborators"`
		Currency                types.String           `tfsdk:"currency"`
		Description             types.String           `tfsdk:"description"`
		EndPeriod               types.Int64            `tfsdk:"end_period"`
		GrowthPerPeriod         types.Float64          `tfsdk:"growth_per_period"`
		Id                      types.String           `tfsdk:"id"` //nolint:revive // Field name must match old state schema
		Metric                  types.String           `tfsdk:"metric"`
		Name                    types.String           `tfsdk:"name"`
		Public                  types.String           `tfsdk:"public"`
		Recipients              types.List             `tfsdk:"recipients"`
		RecipientsSlackChannels []oldSlackChannelModel `tfsdk:"recipients_slack_channels"`
		Scope                   types.List             `tfsdk:"scope"`
		StartPeriod             types.Int64            `tfsdk:"start_period"`
		TimeInterval            types.String           `tfsdk:"time_interval"`
		Type                    types.String           `tfsdk:"type"`
		UsePrevSpend            types.Bool             `tfsdk:"use_prev_spend"`
		LastUpdated             types.String           `tfsdk:"last_updated"`
	}

	var oldState oldBudgetState
	diags := req.State.Get(ctx, &oldState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new state model
	newModel := resource_budget.BudgetModel{
		// Copy simple fields directly
		Id:              oldState.Id,
		Name:            oldState.Name,
		Currency:        oldState.Currency,
		Description:     oldState.Description,
		EndPeriod:       oldState.EndPeriod,
		GrowthPerPeriod: oldState.GrowthPerPeriod,
		Metric:          oldState.Metric,
		Public:          oldState.Public,
		StartPeriod:     oldState.StartPeriod,
		TimeInterval:    oldState.TimeInterval,
		Type:            oldState.Type,
		UsePrevSpend:    oldState.UsePrevSpend,
		Amount:          oldState.Amount,
		Recipients:      oldState.Recipients,
		Scope:           oldState.Scope,
	}

	// Transform alerts list - add new computed fields
	// Old schema: only percentage field
	// New schema: percentage, forecasted_date, triggered (both computed, will be populated on next read)
	if len(oldState.Alerts) > 0 {
		alertElements := make([]attr.Value, 0, len(oldState.Alerts))
		for _, alert := range oldState.Alerts {
			alertElements = append(alertElements, types.ObjectValueMust(
				map[string]attr.Type{
					"percentage":      types.Float64Type,
					"forecasted_date": types.Int64Type,
					"triggered":       types.BoolType,
				},
				map[string]attr.Value{
					"percentage":      alert.Percentage,
					"forecasted_date": types.Int64Null(), // Will be populated on next read
					"triggered":       types.BoolNull(),  // Will be populated on next read
				},
			))
		}
		newModel.Alerts = types.ListValueMust(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"percentage":      types.Float64Type,
					"forecasted_date": types.Int64Type,
					"triggered":       types.BoolType,
				},
			},
			alertElements,
		)
	} else {
		// Empty list, not null — preserves user's config of alerts = []
		newModel.Alerts = types.ListValueMust(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"percentage":      types.Float64Type,
				"forecasted_date": types.Int64Type,
				"triggered":       types.BoolType,
			},
		}, []attr.Value{})
	}

	// Transform collaborators list
	if len(oldState.Collaborators) > 0 {
		collabElements := make([]attr.Value, 0, len(oldState.Collaborators))
		for _, collab := range oldState.Collaborators {
			collabElements = append(collabElements, types.ObjectValueMust(
				map[string]attr.Type{
					"email": types.StringType,
					"role":  types.StringType,
				},
				map[string]attr.Value{
					"email": collab.Email,
					"role":  collab.Role,
				},
			))
		}
		newModel.Collaborators = types.ListValueMust(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"email": types.StringType,
					"role":  types.StringType,
				},
			},
			collabElements,
		)
	} else {
		// Empty list, not null — preserves user's config of collaborators = []
		newModel.Collaborators = types.ListValueMust(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"email": types.StringType,
				"role":  types.StringType,
			},
		}, []attr.Value{})
	}

	// Transform slack channels list
	if len(oldState.RecipientsSlackChannels) > 0 {
		slackElements := make([]attr.Value, 0, len(oldState.RecipientsSlackChannels))
		for _, slack := range oldState.RecipientsSlackChannels {
			slackElements = append(slackElements, types.ObjectValueMust(
				map[string]attr.Type{
					"customer_id": types.StringType,
					"id":          types.StringType,
					"name":        types.StringType,
					"shared":      types.BoolType,
					"type":        types.StringType,
					"workspace":   types.StringType,
				},
				map[string]attr.Value{
					"customer_id": slack.CustomerId,
					"id":          slack.Id,
					"name":        slack.Name,
					"shared":      slack.Shared,
					"type":        slack.Type,
					"workspace":   slack.Workspace,
				},
			))
		}
		newModel.RecipientsSlackChannels = types.ListValueMust(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"customer_id": types.StringType,
					"id":          types.StringType,
					"name":        types.StringType,
					"shared":      types.BoolType,
					"type":        types.StringType,
					"workspace":   types.StringType,
				},
			},
			slackElements,
		)
	} else {
		// Empty list, not null — preserves user's config of recipients_slack_channels = []
		newModel.RecipientsSlackChannels = types.ListValueMust(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"customer_id": types.StringType,
				"id":          types.StringType,
				"name":        types.StringType,
				"shared":      types.BoolType,
				"type":        types.StringType,
				"workspace":   types.StringType,
			},
		}, []attr.Value{})
	}

	// Initialize new fields — use empty list instead of null for consistency.
	// These fields are new in V1 and don't exist in V0 state, but empty list is safer
	// than null because subsequent Read will populate them from the API.
	newModel.SeasonalAmounts = types.ListValueMust(types.Float64Type, []attr.Value{})
	newModel.Scopes = types.ListValueMust(types.ObjectType{
		AttrTypes: resource_budget.ScopesValue{}.AttributeTypes(ctx),
	}, []attr.Value{})
	newModel.CreateTime = types.Int64Null()
	newModel.UpdateTime = types.Int64Null()
	newModel.CurrentUtilization = types.Float64Null()
	newModel.ForecastedUtilization = types.Float64Null()

	// Set the entire upgraded state at once
	diags = resp.State.Set(ctx, newModel)
	resp.Diagnostics.Append(diags...)
}
