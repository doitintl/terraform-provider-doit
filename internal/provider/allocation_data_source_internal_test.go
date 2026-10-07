package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_allocation"
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

func TestAllocationDataSource_MapValueExtraction_NilProvidersReturnsEmptyList(t *testing.T) {
	ctx := t.Context()
	ds := &allocationDataSource{}

	ve := models.AllocationValueExtraction{
		Sources: []models.AllocationValueExtractionSource{
			{
				Key:       "label-key",
				Type:      models.AllocationValueExtractionSourceTypeLabel,
				Providers: nullable.NewNullNullable[[]string](),
			},
		},
	}

	veVal, diags := ds.mapValueExtraction(ctx, valueToNullable(ve))
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}

	var sources []datasource_allocation.SourcesValue
	diags = veVal.Sources.ElementsAs(ctx, &sources, false)
	if diags.HasError() {
		t.Fatalf("failed to unpack sources: %v", diags)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Providers.IsNull() {
		t.Fatalf("expected non-null providers list for empty/null API providers, got null")
	}
	if len(sources[0].Providers.Elements()) != 0 {
		t.Fatalf("expected empty providers list (length 0), got %d elements", len(sources[0].Providers.Elements()))
	}
}
