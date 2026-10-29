package provider

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_commitments"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCommitmentsDataSourceSchema_ValidateImplementation(t *testing.T) {
	ctx := t.Context()
	ds := NewCommitmentsDataSource()

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("Commitments schema validation failed: %v", diags)
	}
}

func TestMapCommitmentsListPeriods_EmptyReturnsNonNullList(t *testing.T) {
	ctx := t.Context()

	for _, tc := range []struct {
		name    string
		periods *[]models.CommitmentPeriod
	}{
		{name: "nil slice", periods: nil},
		{name: "empty slice", periods: &[]models.CommitmentPeriod{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			list, diags := mapCommitmentsListPeriods(ctx, tc.periods)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if list.IsNull() {
				t.Fatal("expected non-null empty list, got null")
			}
			if list.IsUnknown() {
				t.Fatal("expected known list, got unknown")
			}
			if got := len(list.Elements()); got != 0 {
				t.Fatalf("expected 0 elements, got %d", got)
			}
		})
	}
}

func TestMapCommitmentListItem(t *testing.T) {
	ctx := t.Context()

	t.Run("fully populated item and periods", func(t *testing.T) {
		start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		mid := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		provider := models.CommitmentExternalListItemCloudProvider("google-cloud")

		item := models.CommitmentExternalListItem{
			Id:                     new("comm-1"),
			Name:                   new("Commitment 1"),
			CloudProvider:          &provider,
			Currency:               new("USD"),
			CreateTime:             new(int64(1704067200000)),
			UpdateTime:             new(int64(1704153600000)),
			StartDate:              &start,
			EndDate:                &end,
			TotalCommitmentValue:   new(200000.0),
			TotalCurrentAttainment: new(195000.0),
			TotalForecastValue:     new(210000.0),
			TotalMarketplaceSpend:  new(20000.0),
			Periods: &[]models.CommitmentPeriod{
				{
					CashAttainment:             new(110000.0),
					CommitmentValue:            new(100000.0),
					EndDate:                    &mid,
					ForecastValue:              new(110000.0),
					MarketplaceLimitAmount:     new(25000.0),
					MarketplaceLimitPercentage: new(25.0),
					MarketplaceSpend:           new(10000.0),
					RolloverFromPrevPeriod:     new(0.0),
					ShortfallRollover:          new(0.0),
					StartDate:                  &start,
				},
				{
					CashAttainment:             new(85000.0),
					CommitmentValue:            new(100000.0),
					EndDate:                    &end,
					ForecastValue:              new(100000.0),
					MarketplaceLimitAmount:     new(25000.0),
					MarketplaceLimitPercentage: new(25.0),
					MarketplaceSpend:           new(10000.0),
					RolloverFromPrevPeriod:     new(10000.0),
					ShortfallRollover:          new(0.0),
					StartDate:                  &mid,
				},
			},
		}

		cv, diags := mapCommitmentListItem(ctx, item)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if cv.Id.ValueString() != "comm-1" {
			t.Errorf("id = %q, want %q", cv.Id.ValueString(), "comm-1")
		}
		if cv.TotalCurrentAttainment.ValueFloat64() != 195000.0 {
			t.Errorf("total_current_attainment = %v, want 195000.0", cv.TotalCurrentAttainment.ValueFloat64())
		}

		var periods []datasource_commitments.PeriodsValue
		if d := cv.Periods.ElementsAs(ctx, &periods, false); d.HasError() {
			t.Fatalf("ElementsAs failed: %v", d)
		}
		if len(periods) != 2 {
			t.Fatalf("expected 2 periods, got %d", len(periods))
		}

		if periods[0].CashAttainment.ValueFloat64() != 110000.0 {
			t.Errorf("periods[0].cash_attainment = %v, want 110000.0", periods[0].CashAttainment.ValueFloat64())
		}
		if periods[0].RolloverFromPrevPeriod.ValueFloat64() != 0.0 {
			t.Errorf("periods[0].rollover_from_prev_period = %v, want 0.0", periods[0].RolloverFromPrevPeriod.ValueFloat64())
		}
		if periods[0].ShortfallRollover.ValueFloat64() != 0.0 {
			t.Errorf("periods[0].shortfall_rollover = %v, want 0.0", periods[0].ShortfallRollover.ValueFloat64())
		}

		if periods[1].CashAttainment.ValueFloat64() != 85000.0 {
			t.Errorf("periods[1].cash_attainment = %v, want 85000.0", periods[1].CashAttainment.ValueFloat64())
		}
		if periods[1].RolloverFromPrevPeriod.ValueFloat64() != 10000.0 {
			t.Errorf("periods[1].rollover_from_prev_period = %v, want 10000.0", periods[1].RolloverFromPrevPeriod.ValueFloat64())
		}
		if periods[1].ShortfallRollover.ValueFloat64() != 0.0 {
			t.Errorf("periods[1].shortfall_rollover = %v, want 0.0", periods[1].ShortfallRollover.ValueFloat64())
		}
	})

	t.Run("omitted fields map to null and nil periods map to empty list", func(t *testing.T) {
		item := models.CommitmentExternalListItem{
			Periods: &[]models.CommitmentPeriod{{}},
		}

		cv, diags := mapCommitmentListItem(ctx, item)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if !cv.Id.IsNull() {
			t.Errorf("expected null id, got %v", cv.Id)
		}
		if !cv.TotalCurrentAttainment.IsNull() {
			t.Errorf("expected null total_current_attainment, got %v", cv.TotalCurrentAttainment)
		}

		var periods []datasource_commitments.PeriodsValue
		if d := cv.Periods.ElementsAs(ctx, &periods, false); d.HasError() {
			t.Fatalf("ElementsAs failed: %v", d)
		}
		if len(periods) != 1 {
			t.Fatalf("expected 1 period, got %d", len(periods))
		}
		if !periods[0].CashAttainment.IsNull() {
			t.Errorf("expected null cash_attainment, got %v", periods[0].CashAttainment)
		}
		if !periods[0].RolloverFromPrevPeriod.IsNull() {
			t.Errorf("expected null rollover_from_prev_period, got %v", periods[0].RolloverFromPrevPeriod)
		}
		if !periods[0].ShortfallRollover.IsNull() {
			t.Errorf("expected null shortfall_rollover, got %v", periods[0].ShortfallRollover)
		}
	})
}

func TestCommitmentsDataSource_UnknownGuard(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("API should not have been called when filter is unknown, received %s %s", r.Method, r.URL)
	}))

	resp := readCommitments(t, server, map[string]tftypes.Value{
		"filter": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state commitmentsDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if !state.Commitments.IsUnknown() {
		t.Errorf("expected Commitments to be unknown, got %v", state.Commitments)
	}
}

func TestCommitmentsDataSource_Read(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantErr    bool
		errPattern string
		wantCount  int
	}{
		{
			name:   "successful list with period cash attainment and rollover fields",
			status: http.StatusOK,
			body: `{
				"commitments": [
					{
						"id": "comm-123",
						"name": "GCP Commitment",
						"cloudProvider": "google-cloud",
						"currency": "USD",
						"totalCommitmentValue": 100000.0,
						"totalCurrentAttainment": 80000.0,
						"totalForecastValue": 95000.0,
						"totalMarketplaceSpend": 5000.0,
						"periods": [
							{
								"startDate": "2024-01-01T00:00:00Z",
								"endDate": "2025-01-01T00:00:00Z",
								"commitmentValue": 100000.0,
								"cashAttainment": 80000.0,
								"rolloverFromPrevPeriod": 5000.0,
								"shortfallRollover": 15000.0,
								"forecastValue": 95000.0,
								"marketplaceSpend": 5000.0,
								"marketplaceLimitAmount": 25000.0,
								"marketplaceLimitPercentage": 25.0
							}
						]
					}
				],
				"rowCount": 1
			}`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:       "server error 500",
			status:     http.StatusInternalServerError,
			body:       `{"status": 500, "title": "Internal Server Error"}`,
			wantErr:    true,
			errPattern: "Status: 500",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/analytics/v1/commitment-manager" {
					t.Errorf("unexpected request: %s %s, want GET /analytics/v1/commitment-manager", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			resp := readCommitments(t, server, nil)
			if tc.wantErr {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected error containing %q, got none", tc.errPattern)
				}
				found := false
				for _, d := range resp.Diagnostics.Errors() {
					if strings.Contains(d.Summary(), tc.errPattern) || strings.Contains(d.Detail(), tc.errPattern) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected error containing %q, got %v", tc.errPattern, resp.Diagnostics)
				}
				return
			}

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			var state commitmentsDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if state.RowCount.ValueInt64() != int64(tc.wantCount) {
				t.Errorf("row_count = %d, want %d", state.RowCount.ValueInt64(), tc.wantCount)
			}

			var items []datasource_commitments.CommitmentsValue
			if diags := state.Commitments.ElementsAs(t.Context(), &items, false); diags.HasError() {
				t.Fatal(diags)
			}
			if len(items) != tc.wantCount {
				t.Fatalf("expected %d items, got %d", tc.wantCount, len(items))
			}

			var periods []datasource_commitments.PeriodsValue
			if diags := items[0].Periods.ElementsAs(t.Context(), &periods, false); diags.HasError() {
				t.Fatal(diags)
			}
			if len(periods) != 1 {
				t.Fatalf("expected 1 period, got %d", len(periods))
			}

			if periods[0].CashAttainment.ValueFloat64() != 80000.0 {
				t.Errorf("periods[0].cash_attainment = %v, want 80000.0", periods[0].CashAttainment.ValueFloat64())
			}
			if periods[0].RolloverFromPrevPeriod.ValueFloat64() != 5000.0 {
				t.Errorf("periods[0].rollover_from_prev_period = %v, want 5000.0", periods[0].RolloverFromPrevPeriod.ValueFloat64())
			}
			if periods[0].ShortfallRollover.ValueFloat64() != 15000.0 {
				t.Errorf("periods[0].shortfall_rollover = %v, want 15000.0", periods[0].ShortfallRollover.ValueFloat64())
			}
		})
	}
}

func readCommitments(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewCommitmentsDataSource()
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
		values[name] = tftypes.NewValue(tfType, nil)
	}
	maps.Copy(values, overrides)

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
