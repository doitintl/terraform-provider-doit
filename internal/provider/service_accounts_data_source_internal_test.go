package provider

import (
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_service_accounts"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

func TestMapServiceAccountsToModel(t *testing.T) {
	created := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	items := []models.ServiceAccount{{
		Id: valueToNullable("sa-1"), CustomerId: new("cust-1"),
		Name: "terraform-ci", Description: "CI account",
		Permissions: []string{"budgetsReadOnly"},
		CreateTime:  valueToNullable(created),
	}}
	var data serviceAccountsDataSourceModel
	if diags := mapServiceAccountsToModel(t.Context(), items, &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Items.IsNull() || len(data.Items.Elements()) != 1 {
		t.Fatalf("items = %v, want one item", data.Items)
	}
	value, ok := data.Items.Elements()[0].(datasource_service_accounts.ItemsValue)
	if !ok {
		t.Fatalf("item has type %T", data.Items.Elements()[0])
	}
	if value.Id.ValueString() != "sa-1" || value.CustomerId.ValueString() != "cust-1" || value.CreateTime.ValueString() != "2026-09-01T08:00:00Z" {
		t.Fatalf("mapped item = %v", value)
	}
	if !value.CreatedBy.IsNull() || value.Permissions.IsNull() || len(value.Permissions.Elements()) != 1 {
		t.Fatalf("nullable metadata or permissions mapped incorrectly: %v", value)
	}
}

func TestMapServiceAccountsToModel_EmptyAndMissingID(t *testing.T) {
	var data serviceAccountsDataSourceModel
	if diags := mapServiceAccountsToModel(t.Context(), nil, &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Items.IsNull() || len(data.Items.Elements()) != 0 {
		t.Fatalf("items = %v, want empty non-null list", data.Items)
	}
	if diags := mapServiceAccountsToModel(t.Context(), []models.ServiceAccount{{Name: "missing-id"}}, &data); !diags.HasError() {
		t.Fatal("expected diagnostic for missing ID")
	}
}
