package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/oapi-codegen/nullable"
)

func TestAllocationDataSource_MapAllocationToModel_ChildAllocationErrorReturnsDiags(t *testing.T) {
	ctx := t.Context()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	client := newTestAPIClient(t, server)
	ds := &allocationDataSource{client: client}

	ruleID := "child-rule-1"
	ruleName := "child-rule-name"
	allocType := models.AllocationAllocationTypeGroup
	alloc := &models.Allocation{
		Id:             new("group-1"),
		Name:           new("group-name"),
		AllocationType: &allocType,
		Rules: &[]nullable.Nullable[models.GroupAllocationRule]{
			valueToNullable(models.GroupAllocationRule{
				Id:   &ruleID,
				Name: &ruleName,
				// Formula is nil, so it attempts to fetch child allocation
			}),
		},
	}

	var data allocationDataSourceModel
	diags := ds.mapAllocationToModel(ctx, alloc, &data)

	if !diags.HasError() {
		t.Fatalf("expected error diagnostic when child allocation fetch fails with 500, got none")
	}
}

func TestAllocationDataSource_MapAllocationToModel_ChildAllocationMalformedValueExtraction_ReturnsDiags(t *testing.T) {
	ctx := t.Context()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"child-rule-1","name":"child-rule-name","rule":{"formula":"A","components":[],"valueExtraction":{"sources":"invalid-not-an-array"}}}`))
	}))

	client := newTestAPIClient(t, server)
	ds := &allocationDataSource{client: client}

	ruleID := "child-rule-1"
	ruleName := "child-rule-name"
	allocType := models.AllocationAllocationTypeGroup
	alloc := &models.Allocation{
		Id:             new("group-1"),
		Name:           new("group-name"),
		AllocationType: &allocType,
		Rules: &[]nullable.Nullable[models.GroupAllocationRule]{
			valueToNullable(models.GroupAllocationRule{
				Id:   &ruleID,
				Name: &ruleName,
				// Formula is nil, so it attempts to fetch child allocation
			}),
		},
	}

	var data allocationDataSourceModel
	diags := ds.mapAllocationToModel(ctx, alloc, &data)

	if !diags.HasError() {
		t.Fatalf("expected error diagnostic when child allocation valueExtraction is malformed, got none")
	}
}
