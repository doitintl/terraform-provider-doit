package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_planned_purchases"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestMapGcpPlannedPurchasesToItemsList_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	startDate := openapi_types.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	endDate := openapi_types.Date{Time: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)}
	stepDate := openapi_types.Date{Time: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)}

	purchases := []models.GcpPlannedPurchaseByService{
		{
			BillingAccountId: "012345-6789AB-CDEF01",
			Service:          models.GcpPlannedPurchaseByServiceService("compute"),
			Regions: []models.GcpPlannedPurchase{
				{
					Region:                 "global",
					Status:                 models.GcpPlannedPurchaseStatusValid,
					PurchaseApprovalStatus: new(models.GcpPlannedPurchasePurchaseApprovalStatus("approved")),
					PauseNote:              valueToNullable("Paused for testing"),
					RequiresApproval:       new(false),
					WowViolation:           new(true),
					Profile:                new(models.CommitmentPolicyId("balanced")),
					Term:                   new(models.CommitmentTerm("1_year")),
					WeeksToTarget:          valueToNullable(4),
					PlanningCycleStartDate: valueToNullable(startDate),
					PlanningCycleEndDate:   valueToNullable(endDate),
					FinalCommitment: &models.Money{
						Amount:   "100.00",
						Currency: "USD",
					},
					EstimatedSavings: &models.Money{
						Amount:   "250.50",
						Currency: "USD",
					},
					Steps: &[]models.GcpPurchasePlanStep{
						{
							Order:            1,
							ScheduledDate:    stepDate,
							IsBootstrap:      true,
							IsFinal:          false,
							RequiresApproval: false,
							PurchaseAmount: models.Money{
								Amount:   "50.00",
								Currency: "USD",
							},
							CumulativeCommitment: models.Money{
								Amount:   "50.00",
								Currency: "USD",
							},
							EstimatedSavings: models.Money{
								Amount:   "125.25",
								Currency: "USD",
							},
						},
					},
				},
			},
		},
	}

	list, diags := mapGcpPlannedPurchasesToItemsList(ctx, purchases)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}

	item, ok := elements[0].(datasource_ps4c_gcp_planned_purchases.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", elements[0])
	}

	if got := item.BillingAccountId.ValueString(); got != "012345-6789AB-CDEF01" {
		t.Errorf("billing_account_id = %q, want %q", got, "012345-6789AB-CDEF01")
	}
	if got := item.Service.ValueString(); got != "compute" {
		t.Errorf("service = %q, want %q", got, "compute")
	}

	regionsList := item.Regions.Elements()
	if len(regionsList) != 1 {
		t.Fatalf("regions has %d elements, want 1", len(regionsList))
	}
	region0, ok := regionsList[0].(datasource_ps4c_gcp_planned_purchases.RegionsValue)
	if !ok {
		t.Fatalf("regions[0] has unexpected type %T", regionsList[0])
	}

	if got := region0.Region.ValueString(); got != "global" {
		t.Errorf("region = %q, want %q", got, "global")
	}
	if got := region0.Status.ValueString(); got != "valid" {
		t.Errorf("status = %q, want %q", got, "valid")
	}
	if got := region0.PurchaseApprovalStatus.ValueString(); got != "approved" {
		t.Errorf("purchase_approval_status = %q, want %q", got, "approved")
	}
	if got := region0.PauseNote.ValueString(); got != "Paused for testing" {
		t.Errorf("pause_note = %q, want %q", got, "Paused for testing")
	}
	if got := region0.RequiresApproval.ValueBool(); got != false {
		t.Errorf("requires_approval = %v, want false", got)
	}
	if got := region0.WowViolation.ValueBool(); got != true {
		t.Errorf("wow_violation = %v, want true", got)
	}
	if got := region0.Profile.ValueString(); got != "balanced" {
		t.Errorf("profile = %q, want %q", got, "balanced")
	}
	if got := region0.Term.ValueString(); got != "1_year" {
		t.Errorf("term = %q, want %q", got, "1_year")
	}
	if got := region0.WeeksToTarget.ValueInt64(); got != 4 {
		t.Errorf("weeks_to_target = %d, want 4", got)
	}
	if got := region0.PlanningCycleStartDate.ValueString(); got != "2025-01-01" {
		t.Errorf("planning_cycle_start_date = %q, want %q", got, "2025-01-01")
	}
	if got := region0.PlanningCycleEndDate.ValueString(); got != "2025-06-01" {
		t.Errorf("planning_cycle_end_date = %q, want %q", got, "2025-06-01")
	}

	if got := region0.FinalCommitment.Amount.ValueString(); got != "100.00" {
		t.Errorf("final_commitment.amount = %q, want %q", got, "100.00")
	}
	if got := region0.FinalCommitment.Currency.ValueString(); got != "USD" {
		t.Errorf("final_commitment.currency = %q, want %q", got, "USD")
	}
	if got := region0.EstimatedSavings.Amount.ValueString(); got != "250.50" {
		t.Errorf("estimated_savings.amount = %q, want %q", got, "250.50")
	}
	if got := region0.EstimatedSavings.Currency.ValueString(); got != "USD" {
		t.Errorf("estimated_savings.currency = %q, want %q", got, "USD")
	}

	stepsList := region0.Steps.Elements()
	if len(stepsList) != 1 {
		t.Fatalf("steps has %d elements, want 1", len(stepsList))
	}
	step0, ok := stepsList[0].(datasource_ps4c_gcp_planned_purchases.StepsValue)
	if !ok {
		t.Fatalf("steps[0] has unexpected type %T", stepsList[0])
	}
	if got := step0.Order.ValueInt64(); got != 1 {
		t.Errorf("step.order = %d, want 1", got)
	}
	if got := step0.ScheduledDate.ValueString(); got != "2025-01-15" {
		t.Errorf("step.scheduled_date = %q, want %q", got, "2025-01-15")
	}
	if got := step0.IsBootstrap.ValueBool(); got != true {
		t.Errorf("step.is_bootstrap = %v, want true", got)
	}
	if got := step0.IsFinal.ValueBool(); got != false {
		t.Errorf("step.is_final = %v, want false", got)
	}
	if got := step0.RequiresApproval.ValueBool(); got != false {
		t.Errorf("step.requires_approval = %v, want false", got)
	}
	if got := step0.PurchaseAmount.Amount.ValueString(); got != "50.00" {
		t.Errorf("step.purchase_amount.amount = %q, want %q", got, "50.00")
	}
	if got := step0.CumulativeCommitment.Amount.ValueString(); got != "50.00" {
		t.Errorf("step.cumulative_commitment.amount = %q, want %q", got, "50.00")
	}
	if got := step0.EstimatedSavings.Amount.ValueString(); got != "125.25" {
		t.Errorf("step.estimated_savings.amount = %q, want %q", got, "125.25")
	}
}

func TestMapGcpPlannedPurchasesToItemsList_EmptyAndNulls(t *testing.T) {
	ctx := t.Context()

	purchases := []models.GcpPlannedPurchaseByService{
		{
			BillingAccountId: "012345-6789AB-CDEF01",
			Service:          models.GcpPlannedPurchaseByServiceService("cloud_sql"),
			Regions: []models.GcpPlannedPurchase{
				{
					Region: "us_east1",
					Status: models.GcpPlannedPurchaseStatusValid,
					Steps:  nil,
				},
			},
		},
	}

	list, diags := mapGcpPlannedPurchasesToItemsList(ctx, purchases)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}
	item := elements[0].(datasource_ps4c_gcp_planned_purchases.ItemsValue)
	regions := item.Regions.Elements()
	if len(regions) != 1 {
		t.Fatalf("regions has %d elements, want 1", len(regions))
	}
	reg := regions[0].(datasource_ps4c_gcp_planned_purchases.RegionsValue)
	if !reg.Steps.IsNull() && len(reg.Steps.Elements()) != 0 {
		t.Errorf("expected steps to be empty, got %d", len(reg.Steps.Elements()))
	}
	if !reg.PauseNote.IsNull() {
		t.Errorf("expected pause_note to be null, got %v", reg.PauseNote)
	}
	if !reg.PlanningCycleStartDate.IsNull() {
		t.Errorf("expected planning_cycle_start_date to be null, got %v", reg.PlanningCycleStartDate)
	}
	if !reg.PlanningCycleEndDate.IsNull() {
		t.Errorf("expected planning_cycle_end_date to be null, got %v", reg.PlanningCycleEndDate)
	}
	if !reg.FinalCommitment.IsNull() {
		t.Errorf("expected final_commitment to be null, got %v", reg.FinalCommitment)
	}
	if !reg.EstimatedSavings.IsNull() {
		t.Errorf("expected estimated_savings to be null, got %v", reg.EstimatedSavings)
	}
}

func TestPs4cGcpPlannedPurchasesRead(t *testing.T) {
	for _, tc := range []struct {
		name        string
		gcpService  *string
		region      *string
		maxResults  *int64
		pageToken   *string
		responses   []string
		wantCalls   int
		wantItems   int
		wantCount   int64
		wantCursor  string
		checkParams func(t *testing.T, r *http.Request)
	}{
		{
			name:       "auto-pagination single page with service and region filters",
			gcpService: new("compute"),
			region:     new("global"),
			responses: []string{
				`{
					"items": [
						{
							"billingAccountId": "012345-6789AB-CDEF01",
							"service": "compute",
							"regions": [
								{
									"region": "global",
									"status": "valid"
								}
							]
						}
					],
					"rowCount": 1
				}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 1,
			checkParams: func(t *testing.T, r *http.Request) {
				if q := r.URL.Query().Get("gcp_service"); q != "compute" {
					t.Errorf("gcp_service query param = %q, want compute", q)
				}
				if q := r.URL.Query().Get("region"); q != "global" {
					t.Errorf("region query param = %q, want global", q)
				}
			},
		},
		{
			name: "auto-pagination multiple pages",
			responses: []string{
				`{
					"items": [
						{"billingAccountId": "012345-6789AB-CDEF01", "service": "compute", "regions": []}
					],
					"pageToken": "page-2",
					"rowCount": 1
				}`,
				`{
					"items": [
						{"billingAccountId": "012345-6789AB-CDEF01", "service": "cloud_sql", "regions": []}
					],
					"pageToken": null,
					"rowCount": 1
				}`,
			},
			wantCalls: 2,
			wantItems: 2,
			wantCount: 2,
		},
		{
			name:       "user-controlled pagination with max_results only",
			maxResults: new(int64(1)),
			responses: []string{
				`{
					"items": [
						{"billingAccountId": "012345-6789AB-CDEF01", "service": "compute", "regions": []}
					],
					"pageToken": "next-cursor",
					"rowCount": 10
				}`,
			},
			wantCalls:  1,
			wantItems:  1,
			wantCount:  10,
			wantCursor: "next-cursor",
			checkParams: func(t *testing.T, r *http.Request) {
				if q := r.URL.Query().Get("maxResults"); q != "1" {
					t.Errorf("maxResults query param = %q, want 1", q)
				}
				if q := r.URL.Query().Get("pageToken"); q != "" {
					t.Errorf("pageToken query param = %q, want empty", q)
				}
			},
		},
		{
			name:       "user-controlled pagination with both max_results and page_token",
			maxResults: new(int64(1)),
			pageToken:  new("current-cursor"),
			responses: []string{
				`{
					"items": [
						{"billingAccountId": "012345-6789AB-CDEF01", "service": "cloud_sql", "regions": []}
					],
					"pageToken": "after-cursor",
					"rowCount": 5
				}`,
			},
			wantCalls:  1,
			wantItems:  1,
			wantCount:  5,
			wantCursor: "after-cursor",
			checkParams: func(t *testing.T, r *http.Request) {
				if q := r.URL.Query().Get("maxResults"); q != "1" {
					t.Errorf("maxResults = %q, want 1", q)
				}
				if q := r.URL.Query().Get("pageToken"); q != "current-cursor" {
					t.Errorf("pageToken = %q, want current-cursor", q)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callIdx := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/planned-purchases" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if tc.checkParams != nil {
					tc.checkParams(t, r)
				}
				if callIdx >= len(tc.responses) {
					t.Fatalf("unexpected call %d", callIdx)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.responses[callIdx]))
				callIdx++
			}))

			overrides := map[string]tftypes.Value{
				"billing_account_id": tftypes.NewValue(tftypes.String, "012345-6789AB-CDEF01"),
			}
			if tc.gcpService != nil {
				overrides["gcp_service"] = tftypes.NewValue(tftypes.String, *tc.gcpService)
			}
			if tc.region != nil {
				overrides["region"] = tftypes.NewValue(tftypes.String, *tc.region)
			}
			if tc.maxResults != nil {
				overrides["max_results"] = tftypes.NewValue(tftypes.Number, float64(*tc.maxResults))
			}
			if tc.pageToken != nil {
				overrides["page_token"] = tftypes.NewValue(tftypes.String, *tc.pageToken)
			}

			resp := readPs4cGcpPlannedPurchases(t, server, overrides)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if callIdx != tc.wantCalls {
				t.Fatalf("got %d calls, want %d", callIdx, tc.wantCalls)
			}

			var state ps4cGcpPlannedPurchasesDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if len(state.Items.Elements()) != tc.wantItems {
				t.Errorf("got %d items, want %d", len(state.Items.Elements()), tc.wantItems)
			}
			if state.RowCount.ValueInt64() != tc.wantCount {
				t.Errorf("got row_count %d, want %d", state.RowCount.ValueInt64(), tc.wantCount)
			}
			if tc.wantCursor != "" {
				if state.PageToken.ValueString() != tc.wantCursor {
					t.Errorf("page_token = %q, want %q", state.PageToken.ValueString(), tc.wantCursor)
				}
			} else {
				if !state.PageToken.IsNull() {
					t.Errorf("page_token = %q, want null", state.PageToken.ValueString())
				}
			}
		})
	}
}

func TestPs4cGcpPlannedPurchasesReadErrors(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))

	resp := readPs4cGcpPlannedPurchases(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "status 500") {
		t.Errorf("expected error containing 'status 500', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpPlannedPurchases(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpPlannedPurchasesDataSource()
	var configureResp datasource.ConfigureResponse
	ds.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatal(configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatal(diags)
	}

	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, attribute := range schemaResp.Schema.Attributes {
		tfType := attribute.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if val, ok := overrides[name]; ok {
			values[name] = val
		} else if name == "billing_account_id" {
			values[name] = tftypes.NewValue(tfType, "012345-6789AB-CDEF01")
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
