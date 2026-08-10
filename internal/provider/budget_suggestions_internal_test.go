package provider

import (
	"context"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_budget_suggestions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// TestMapBudgetSuggestionsItems_FullyPopulated exercises every nested
// mapping path (amount, scope_chips with nested values, config, enums,
// generated_time) with a fully populated API response.
func TestMapBudgetSuggestionsItems_FullyPopulated(t *testing.T) {
	ctx := context.Background()

	generatedTime := time.Date(2026, 8, 1, 12, 30, 0, 0, time.UTC)
	confidence := models.BudgetSuggestionConfidence("high")
	status := models.BudgetSuggestionStatus("pending")

	items := []models.BudgetSuggestion{
		{
			Id:              new("sugg-1"),
			Name:            new("GCP Compute Engine E2 RAM runtime"),
			Description:     new("Suggested budget for E2 RAM"),
			Rationale:       new("Spend trending upward"),
			Confidence:      &confidence,
			Status:          &status,
			TimeInterval:    new("month"),
			GeneratedTime:   &generatedTime,
			AlertThresholds: &[]float64{50, 80, 100},
			Amount: &models.BudgetSuggestionAmount{
				Amount:   new("123.45"),
				Currency: new("USD"),
			},
			ScopeChips: &[]models.BudgetSuggestionScopeChipsItem{
				{
					Key:    new("cloud_provider"),
					Values: &[]string{"google-cloud"},
				},
				{
					Key:    new("service_description"),
					Values: &[]string{"Compute Engine", "Compute Engine (Premium)"},
				},
			},
			Config: &map[string]any{"scope": []any{"a", "b"}},
		},
	}

	list, diags := mapBudgetSuggestionsItems(ctx, items)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}

	item, ok := elements[0].(datasource_budget_suggestions.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", elements[0])
	}

	if got := item.Id.ValueString(); got != "sugg-1" {
		t.Errorf("id = %q, want %q", got, "sugg-1")
	}
	if got := item.Confidence.ValueString(); got != "high" {
		t.Errorf("confidence = %q, want %q", got, "high")
	}
	if got := item.Status.ValueString(); got != "pending" {
		t.Errorf("status = %q, want %q", got, "pending")
	}
	if got := item.GeneratedTime.ValueString(); got != "2026-08-01T12:30:00Z" {
		t.Errorf("generated_time = %q, want %q", got, "2026-08-01T12:30:00Z")
	}

	if item.Amount.IsNull() {
		t.Fatal("amount should not be null when populated")
	}
	if got := item.Amount.Amount.ValueString(); got != "123.45" {
		t.Errorf("amount.amount = %q, want %q", got, "123.45")
	}
	if got := item.Amount.Currency.ValueString(); got != "USD" {
		t.Errorf("amount.currency = %q, want %q", got, "USD")
	}

	alertThresholds := item.AlertThresholds.Elements()
	if len(alertThresholds) != 3 {
		t.Fatalf("alert_thresholds has %d elements, want 3", len(alertThresholds))
	}

	scopeChips := item.ScopeChips.Elements()
	if len(scopeChips) != 2 {
		t.Fatalf("scope_chips has %d elements, want 2", len(scopeChips))
	}
	chip0, ok := scopeChips[0].(datasource_budget_suggestions.ScopeChipsValue)
	if !ok {
		t.Fatalf("scope_chips[0] has unexpected type %T", scopeChips[0])
	}
	if got := chip0.Key.ValueString(); got != "cloud_provider" {
		t.Errorf("scope_chips[0].key = %q, want %q", got, "cloud_provider")
	}
	if got := len(chip0.Values.Elements()); got != 1 {
		t.Fatalf("scope_chips[0].values has %d elements, want 1", got)
	}

	chip1, ok := scopeChips[1].(datasource_budget_suggestions.ScopeChipsValue)
	if !ok {
		t.Fatalf("scope_chips[1] has unexpected type %T", scopeChips[1])
	}
	if got := len(chip1.Values.Elements()); got != 2 {
		t.Fatalf("scope_chips[1].values has %d elements, want 2", got)
	}

	if item.Config.IsNull() {
		t.Fatal("config should not be null when populated")
	}
	if got := item.Config.ValueString(); got == "" {
		t.Error("config should be a non-empty JSON string")
	}
}

// TestMapBudgetSuggestionsItems_NullFields verifies the null-handling paths
// that a live acceptance test can't reliably exercise: the API marks every
// BudgetSuggestion field optional, so a real suggestion can omit amount,
// scope_chips, config, confidence, and status entirely.
func TestMapBudgetSuggestionsItems_NullFields(t *testing.T) {
	ctx := context.Background()

	items := []models.BudgetSuggestion{
		{
			Id: new("sugg-2"),
		},
	}

	list, diags := mapBudgetSuggestionsItems(ctx, items)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	item, ok := list.Elements()[0].(datasource_budget_suggestions.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", list.Elements()[0])
	}

	if !item.Amount.IsNull() {
		t.Error("amount should be null when the API omits it")
	}
	if !item.Confidence.IsNull() {
		t.Error("confidence should be null when the API omits it")
	}
	if !item.Status.IsNull() {
		t.Error("status should be null when the API omits it")
	}
	if !item.Config.IsNull() {
		t.Error("config should be null when the API omits it")
	}
	if item.ScopeChips.IsNull() || len(item.ScopeChips.Elements()) != 0 {
		t.Error("scope_chips should be an empty, non-null list when the API omits it")
	}
	if item.AlertThresholds.IsNull() || len(item.AlertThresholds.Elements()) != 0 {
		t.Error("alert_thresholds should be an empty, non-null list when the API omits it")
	}
}

// TestMapBudgetSuggestionsItems_Empty verifies that an empty result maps to
// an empty (never null) list, per the Computed-Only List Attributes rule.
func TestMapBudgetSuggestionsItems_Empty(t *testing.T) {
	ctx := context.Background()

	list, diags := mapBudgetSuggestionsItems(ctx, []models.BudgetSuggestion{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if list.IsNull() {
		t.Error("expected an empty, non-null list")
	}
	if len(list.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(list.Elements()))
	}
}
