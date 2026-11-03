package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// newTestBudgetPlan returns a plan holding the attributes the create schema
// requires plus one scope, with every other attribute left null.
func newTestBudgetPlan(t *testing.T) *budgetResourceModel {
	t.Helper()
	ctx := t.Context()

	scope, diags := resource_budget.NewScopesValue(
		resource_budget.ScopesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"case_insensitive": types.BoolNull(),
			"id":               types.StringValue("allocation_rule"),
			"include_null":     types.BoolNull(),
			"inverse":          types.BoolNull(),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("allocation_rule"),
			"values":           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("alloc-1")}),
		},
	)
	if diags.HasError() {
		t.Fatalf("NewScopesValue: %v", diags)
	}
	scopes, diags := types.ListValueFrom(ctx, resource_budget.ScopesValue{}.Type(ctx), []resource_budget.ScopesValue{scope})
	if diags.HasError() {
		t.Fatalf("ListValueFrom: %v", diags)
	}

	plan := &budgetResourceModel{}
	plan.Name = types.StringValue("budget")
	plan.Currency = types.StringValue("USD")
	plan.Type = types.StringValue("recurring")
	plan.StartPeriod = types.Int64Value(1759276800000)
	plan.TimeInterval = types.StringValue("month")
	plan.Metric = types.StringValue("amortized_cost")
	plan.Public = types.StringValue("viewer")
	plan.Scopes = scopes
	return plan
}

func TestBudgetToCreateRequest(t *testing.T) {
	req, diags := newTestBudgetPlan(t).toCreateRequest(t.Context())
	if diags.HasError() {
		t.Fatalf("toCreateRequest: %v", diags)
	}

	if req.Name != "budget" || req.Currency != models.Currency("USD") || req.Type != "recurring" || req.StartPeriod != 1759276800000 {
		t.Errorf("required fields not copied: %+v", req)
	}
	if len(req.Scopes) != 1 || req.Scopes[0].Id != "allocation_rule" || req.Scopes[0].Mode == nil || *req.Scopes[0].Mode != models.ExternalConfigFilterMode("is") {
		t.Errorf("scopes = %+v", req.Scopes)
	}
	if req.Metric == nil || *req.Metric != models.BudgetCreateRequestMetricAmortizedCost {
		t.Errorf("metric = %v", req.Metric)
	}
	if req.Public == nil || *req.Public != models.BudgetCreateRequestPublicViewer {
		t.Errorf("public = %v", req.Public)
	}
	if req.TimeInterval == nil || *req.TimeInterval != "month" {
		t.Errorf("time_interval = %v", req.TimeInterval)
	}
	if req.Alerts != nil || req.Collaborators != nil || req.Recipients != nil || req.RecipientsSlackChannels != nil || req.SeasonalAmounts != nil || req.Amount != nil || req.EndPeriod != nil {
		t.Errorf("omitted optional fields must stay nil: %+v", req)
	}
}

func TestBudgetToUpdateRequest(t *testing.T) {
	req, diags := newTestBudgetPlan(t).toUpdateRequest(t.Context())
	if diags.HasError() {
		t.Fatalf("toUpdateRequest: %v", diags)
	}

	if req.Name == nil || *req.Name != "budget" ||
		req.Currency == nil || *req.Currency != models.Currency("USD") ||
		req.Type == nil || *req.Type != "recurring" ||
		req.StartPeriod == nil || *req.StartPeriod != 1759276800000 {
		t.Errorf("required fields not sent on update: %+v", req)
	}
	if req.Scopes == nil || len(*req.Scopes) != 1 {
		t.Errorf("scopes = %v", req.Scopes)
	}
	if req.Metric == nil || *req.Metric != models.BudgetUpdateRequestMetricAmortizedCost {
		t.Errorf("metric = %v", req.Metric)
	}
	if req.Public == nil || *req.Public != models.BudgetUpdateRequestPublicViewer {
		t.Errorf("public = %v", req.Public)
	}
	if req.Alerts != nil || req.Collaborators != nil || req.Recipients != nil || req.RecipientsSlackChannels != nil || req.SeasonalAmounts != nil {
		t.Errorf("omitted optional lists must stay nil: %+v", req)
	}
}
