package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// UpgradeState implements the ResourceWithUpgradeState interface.
// This method handles migrating state from schema v0 (old budget_resource.go)
// to schema v1 (new budget_resource_new.go with auto-generated schema).
func (r *budgetResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
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

// upgradeBudgetStateV0ToV1 handles the actual state transformation
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
		CustomerId types.String `tfsdk:"customer_id"`
		Id         types.String `tfsdk:"id"`
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
		Id                      types.String           `tfsdk:"id"`
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

	// Transform alerts list
	// Old schema: only percentage field
	// New schema: percentage, forecasted_date, triggered (both computed)
	var newAlerts types.List
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
		newAlerts = types.ListValueMust(
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
		newAlerts = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"percentage":      types.Float64Type,
				"forecasted_date": types.Int64Type,
				"triggered":       types.BoolType,
			},
		})
	}

	// Transform collaborators list
	var newCollaborators types.List
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
		newCollaborators = types.ListValueMust(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"email": types.StringType,
					"role":  types.StringType,
				},
			},
			collabElements,
		)
	} else {
		newCollaborators = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"email": types.StringType,
				"role":  types.StringType,
			},
		})
	}

	// Transform slack channels list
	var newSlackChannels types.List
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
		newSlackChannels = types.ListValueMust(
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
		newSlackChannels = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"customer_id": types.StringType,
				"id":          types.StringType,
				"name":        types.StringType,
				"shared":      types.BoolType,
				"type":        types.StringType,
				"workspace":   types.StringType,
			},
		})
	}

	// Create the new state structure
	// Most fields can be copied directly
	// The new schema adds seasonal_amounts which we initialize as null
	// The last_updated field is dropped (doesn't exist in new schema)
	newState := map[string]attr.Value{
		"alerts":                    newAlerts,
		"amount":                    oldState.Amount,
		"budget":                    types.ObjectNull(map[string]attr.Type{}), // Computed field, will be populated on read
		"collaborators":             newCollaborators,
		"currency":                  oldState.Currency,
		"description":               oldState.Description,
		"end_period":                oldState.EndPeriod,
		"growth_per_period":         oldState.GrowthPerPeriod,
		"id":                        oldState.Id,
		"metric":                    oldState.Metric,
		"name":                      oldState.Name,
		"public":                    oldState.Public,
		"recipients":                oldState.Recipients,
		"recipients_slack_channels": newSlackChannels,
		"scope":                     oldState.Scope,
		"seasonal_amounts":          types.ListNull(types.Float64Type), // New field in v1
		"start_period":              oldState.StartPeriod,
		"time_interval":             oldState.TimeInterval,
		"type":                      oldState.Type,
		"use_prev_spend":            oldState.UsePrevSpend,
		// last_updated is intentionally NOT included (doesn't exist in new schema)
	}

	// Set the upgraded state
	for key, value := range newState {
		setDiags := resp.State.SetAttribute(ctx, path.Root(key), value)
		resp.Diagnostics.Append(setDiags...)
	}
}
