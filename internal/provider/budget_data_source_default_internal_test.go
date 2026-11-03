package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// With no plan-time default, an omitted data_source is Unknown on create and
// must not reach the API so it can resolve its DataHub-aware default.
func TestBudgetRequests_DataSource(t *testing.T) {
	tests := []struct {
		name       string
		dataSource types.String
		want       *string
	}{
		{"null is omitted", types.StringNull(), nil},
		{"unknown is omitted", types.StringUnknown(), nil},
		{"billing is sent", types.StringValue("billing"), new("billing")},
		{"billing-datahub is sent", types.StringValue("billing-datahub"), new("billing-datahub")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := newTestBudgetPlan(t)
			plan.DataSource = tc.dataSource

			createReq, diags := plan.toCreateRequest(t.Context())
			if diags.HasError() {
				t.Fatalf("toCreateRequest: %v", diags)
			}
			var gotCreate *string
			if createReq.DataSource != nil {
				gotCreate = new(string(*createReq.DataSource))
			}
			assertStringPtr(t, "create", gotCreate, tc.want)

			updateReq, diags := plan.toUpdateRequest(t.Context())
			if diags.HasError() {
				t.Fatalf("toUpdateRequest: %v", diags)
			}
			var gotUpdate *string
			if updateReq.DataSource != nil {
				gotUpdate = new(string(*updateReq.DataSource))
			}
			assertStringPtr(t, "update", gotUpdate, tc.want)
		})
	}
}

func newTestBudgetAPI(dataSource *models.CloudAnalyticsBudgetDataSourceReadOnly) *models.BudgetAPI {
	return &models.BudgetAPI{
		Id:           new("budget-1"),
		Name:         "budget",
		Currency:     models.Currency("USD"),
		Type:         "recurring",
		TimeInterval: "month",
		StartPeriod:  1759276800000,
		DataSource:   dataSource,
	}
}

func TestMapBudgetToModel_DataSource(t *testing.T) {
	t.Run("maps the API value", func(t *testing.T) {
		state := newTestBudgetPlan(t)
		if diags := mapBudgetToModel(t.Context(), newTestBudgetAPI(new(models.CloudAnalyticsBudgetDataSourceReadOnly("billing-datahub"))), state); diags.HasError() {
			t.Fatalf("mapBudgetToModel: %v", diags)
		}
		if got := state.DataSource.ValueString(); got != "billing-datahub" {
			t.Errorf("data_source = %q, want billing-datahub", got)
		}
	})

	t.Run("absent in the API response is null", func(t *testing.T) {
		state := newTestBudgetPlan(t)
		if diags := mapBudgetToModel(t.Context(), newTestBudgetAPI(nil), state); diags.HasError() {
			t.Fatalf("mapBudgetToModel: %v", diags)
		}
		if !state.DataSource.IsNull() {
			t.Errorf("data_source = %v, want null", state.DataSource)
		}
	})
}

func TestOverlayBudgetComputedFields_DataSource(t *testing.T) {
	api := newTestBudgetAPI(new(models.CloudAnalyticsBudgetDataSourceReadOnly("billing-datahub")))

	t.Run("unknown resolves from API response", func(t *testing.T) {
		plan := newTestBudgetPlan(t)
		plan.DataSource = types.StringUnknown()
		if diags := overlayBudgetComputedFields(t.Context(), api, plan); diags.HasError() {
			t.Fatalf("overlayBudgetComputedFields: %v", diags)
		}
		if got := plan.DataSource.ValueString(); got != "billing-datahub" {
			t.Errorf("data_source = %q, want billing-datahub", got)
		}
	})

	t.Run("known plan value wins", func(t *testing.T) {
		plan := newTestBudgetPlan(t)
		plan.DataSource = types.StringValue("billing")
		if diags := overlayBudgetComputedFields(t.Context(), api, plan); diags.HasError() {
			t.Fatalf("overlayBudgetComputedFields: %v", diags)
		}
		if got := plan.DataSource.ValueString(); got != "billing" {
			t.Errorf("data_source = %q, want billing", got)
		}
	})
}
