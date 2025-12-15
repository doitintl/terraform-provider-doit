package provider

import (
	"context"
	"testing"

	"terraform-provider-doit/internal/provider/resource_budget"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestBudgetStateUpgradeV0ToV1(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		oldState    map[string]tftypes.Value
		expectError bool
	}{
		{
			name: "full budget with alerts",
			oldState: map[string]tftypes.Value{
				"id":                tftypes.NewValue(tftypes.String, "budget-123"),
				"name":              tftypes.NewValue(tftypes.String, "Test Budget"),
				"description":       tftypes.NewValue(tftypes.String, "A test budget"),
				"currency":          tftypes.NewValue(tftypes.String, "USD"),
				"amount":            tftypes.NewValue(tftypes.Number, 1000.0),
				"type":              tftypes.NewValue(tftypes.String, "recurring"),
				"time_interval":     tftypes.NewValue(tftypes.String, "month"),
				"start_period":      tftypes.NewValue(tftypes.Number, 1640995200000),
				"growth_per_period": tftypes.NewValue(tftypes.Number, 5.0),
				"use_prev_spend":    tftypes.NewValue(tftypes.Bool, false),
				"metric":            tftypes.NewValue(tftypes.String, "cost"),
				"alerts": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"percentage": tftypes.Number,
						},
					}},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"percentage": tftypes.Number,
							},
						}, map[string]tftypes.Value{
							"percentage": tftypes.NewValue(tftypes.Number, 80.0),
						}),
					},
				),
				"collaborators": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"email": tftypes.String,
							"role":  tftypes.String,
						},
					}},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"email": tftypes.String,
								"role":  tftypes.String,
							},
						}, map[string]tftypes.Value{
							"email": tftypes.NewValue(tftypes.String, "test@example.com"),
							"role":  tftypes.NewValue(tftypes.String, "owner"),
						}),
					},
				),
				"recipients": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.String},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.String, "alerts@example.com"),
					},
				),
				"scope": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.String},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.String, "attribution-id-123"),
					},
				),
				"recipients_slack_channels": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"customer_id": tftypes.String,
							"id":          tftypes.String,
							"name":        tftypes.String,
							"shared":      tftypes.Bool,
							"type":        tftypes.String,
							"workspace":   tftypes.String,
						},
					}},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"customer_id": tftypes.String,
								"id":          tftypes.String,
								"name":        tftypes.String,
								"shared":      tftypes.Bool,
								"type":        tftypes.String,
								"workspace":   tftypes.String,
							},
						}, map[string]tftypes.Value{
							"customer_id": tftypes.NewValue(tftypes.String, "cust-123"),
							"id":          tftypes.NewValue(tftypes.String, "chan-123"),
							"name":        tftypes.NewValue(tftypes.String, "channel-name"),
							"shared":      tftypes.NewValue(tftypes.Bool, true),
							"type":        tftypes.NewValue(tftypes.String, "public"),
							"workspace":   tftypes.NewValue(tftypes.String, "workspace-1"),
						}),
					},
				),
				"end_period":   tftypes.NewValue(tftypes.Number, nil),
				"public":       tftypes.NewValue(tftypes.String, nil),
				"last_updated": tftypes.NewValue(tftypes.String, "2024-01-01T00:00:00Z"),
			},
			expectError: false,
		},
		{
			name: "minimal budget",
			oldState: map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, "budget-456"),
				"name":     tftypes.NewValue(tftypes.String, "Minimal Budget"),
				"currency": tftypes.NewValue(tftypes.String, "EUR"),
				"type":     tftypes.NewValue(tftypes.String, "fixed"),
				"collaborators": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"email": tftypes.String,
							"role":  tftypes.String,
						},
					}},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"email": tftypes.String,
								"role":  tftypes.String,
							},
						}, map[string]tftypes.Value{
							"email": tftypes.NewValue(tftypes.String, "owner@example.com"),
							"role":  tftypes.NewValue(tftypes.String, "owner"),
						}),
					},
				),
				"recipients": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.String},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.String, "notify@example.com"),
					},
				),
				"scope": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.String},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.String, "scope-123"),
					},
				),
				"alerts": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"percentage": tftypes.Number,
						},
					}},
					[]tftypes.Value{},
				),
				"recipients_slack_channels": tftypes.NewValue(
					tftypes.List{ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"customer_id": tftypes.String,
							"id":          tftypes.String,
							"name":        tftypes.String,
							"shared":      tftypes.Bool,
							"type":        tftypes.String,
							"workspace":   tftypes.String,
						},
					}},
					[]tftypes.Value{},
				),
				"amount":            tftypes.NewValue(tftypes.Number, 0.0),
				"description":       tftypes.NewValue(tftypes.String, ""),
				"end_period":        tftypes.NewValue(tftypes.Number, nil),
				"growth_per_period": tftypes.NewValue(tftypes.Number, 0.0),
				"metric":            tftypes.NewValue(tftypes.String, "cost"),
				"public":            tftypes.NewValue(tftypes.String, nil),
				"start_period":      tftypes.NewValue(tftypes.Number, 0),
				"time_interval":     tftypes.NewValue(tftypes.String, ""),
				"use_prev_spend":    tftypes.NewValue(tftypes.Bool, false),
				"last_updated":      tftypes.NewValue(tftypes.String, ""),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a budget resource to get the upgrader.
			r := &budgetResource{}

			// Get the state upgraders.
			upgraders := r.UpgradeState(ctx)

			// Get the v0 to v1 upgrader.
			upgrader, ok := upgraders[0]
			if !ok {
				t.Fatal("No upgrader found for version 0")
			}

			// Create the old state value.
			oldStateType := tftypes.Object{AttributeTypes: getV0StateTypes()}
			oldStateValue := tftypes.NewValue(oldStateType, tt.oldState)

			// Create a new tfsdk.State with the prior schema.
			priorState := tfsdk.State{
				Schema: *upgrader.PriorSchema,
				Raw:    oldStateValue,
			}

			// Create the new state with the current schema.
			newState := tfsdk.State{
				Schema: resource_budget.BudgetResourceSchema(ctx),
			}

			// Create request and response.
			req := resource.UpgradeStateRequest{
				State: &priorState,
			}
			resp := &resource.UpgradeStateResponse{
				State: newState,
			}

			// Execute the upgrade.
			upgrader.StateUpgrader(ctx, req, resp)

			// Check for errors.
			if tt.expectError {
				if !resp.Diagnostics.HasError() {
					t.Error("Expected error but got none")
				}
				return
			}

			if resp.Diagnostics.HasError() {
				t.Fatalf("Unexpected error during upgrade: %v", resp.Diagnostics)
			}

			// Basic validation - ensure we can get the state.
			var upgradedModel resource_budget.BudgetModel
			diags := resp.State.Get(ctx, &upgradedModel)
			if diags.HasError() {
				t.Fatalf("Failed to get upgraded state: %v", diags)
			}

			// Verify ID was preserved.
			expectedIDVal := tt.oldState["id"]
			if !expectedIDVal.IsNull() {
				var expectedID string
				if err := expectedIDVal.As(&expectedID); err == nil {
					actualID := upgradedModel.Id.ValueString()
					if actualID != expectedID {
						t.Errorf("Expected ID '%s', got '%s'", expectedID, actualID)
					}
				}
			}

			// Verify RecipientsSlackChannels were preserved if present in old state
			expectedSlackVal := tt.oldState["recipients_slack_channels"]
			if expectedSlackVal.Type().Is(tftypes.List{ElementType: expectedSlackVal.Type().(tftypes.List).ElementType}) {
				var expectedSlackList []tftypes.Value
				if err := expectedSlackVal.As(&expectedSlackList); err == nil && len(expectedSlackList) > 0 {
					if upgradedModel.RecipientsSlackChannels.IsNull() || upgradedModel.RecipientsSlackChannels.IsUnknown() {
						t.Errorf("Expected RecipientsSlackChannels to be preserved, but got Null/Unknown")
					} else {
						actualSlackList := upgradedModel.RecipientsSlackChannels.Elements()
						if len(actualSlackList) != len(expectedSlackList) {
							t.Errorf("Expected %d slack channels, got %d", len(expectedSlackList), len(actualSlackList))
						}
					}
				}
			}

			t.Logf("Successfully upgraded state for test: %s", tt.name)
		})
	}
}

// getV0StateTypes returns the tftypes.Type map for the v0 schema.
func getV0StateTypes() map[string]tftypes.Type {
	return map[string]tftypes.Type{
		"alerts": tftypes.List{
			ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"percentage": tftypes.Number,
				},
			},
		},
		"amount": tftypes.Number,
		"collaborators": tftypes.List{
			ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"email": tftypes.String,
					"role":  tftypes.String,
				},
			},
		},
		"currency":          tftypes.String,
		"description":       tftypes.String,
		"end_period":        tftypes.Number,
		"growth_per_period": tftypes.Number,
		"id":                tftypes.String,
		"metric":            tftypes.String,
		"name":              tftypes.String,
		"public":            tftypes.String,
		"recipients":        tftypes.List{ElementType: tftypes.String},
		"recipients_slack_channels": tftypes.List{
			ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"customer_id": tftypes.String,
					"id":          tftypes.String,
					"name":        tftypes.String,
					"shared":      tftypes.Bool,
					"type":        tftypes.String,
					"workspace":   tftypes.String,
				},
			},
		},
		"scope":          tftypes.List{ElementType: tftypes.String},
		"start_period":   tftypes.Number,
		"time_interval":  tftypes.String,
		"type":           tftypes.String,
		"use_prev_spend": tftypes.Bool,
		"last_updated":   tftypes.String,
	}
}
