package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_commitment"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCommitmentDataSourceSchema_ValidateImplementation(t *testing.T) {
	ctx := t.Context()
	ds := NewCommitmentDataSource()

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("Commitment schema validation failed: %v", diags)
	}
}

func TestMapCommitmentPeriods(t *testing.T) {
	ctx := t.Context()

	t.Run("nil and empty slices return non-null empty list", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			periods *[]models.CommitmentPeriod
		}{
			{name: "nil slice", periods: nil},
			{name: "empty slice", periods: &[]models.CommitmentPeriod{}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				list, diags := mapCommitmentPeriods(ctx, tc.periods)
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
	})

	t.Run("fully populated and omitted fields", func(t *testing.T) {
		p1Start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		p1End := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		p2Start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		p2End := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		periods := []models.CommitmentPeriod{
			{
				CashAttainment:             new(85000.50),
				CommitmentValue:            new(100000.00),
				EndDate:                    &p1End,
				ForecastValue:              new(92000.00),
				MarketplaceLimitAmount:     new(25000.00),
				MarketplaceLimitPercentage: new(25.0),
				MarketplaceSpend:           new(12000.25),
				RolloverFromPrevPeriod:     new(0.0),
				ShortfallRollover:          new(14999.50),
				StartDate:                  &p1Start,
			},
			{
				CashAttainment:             new(115000.00),
				CommitmentValue:            new(100000.00),
				EndDate:                    &p2End,
				ForecastValue:              new(120000.00),
				MarketplaceLimitAmount:     new(25000.00),
				MarketplaceLimitPercentage: new(25.0),
				MarketplaceSpend:           new(18000.00),
				RolloverFromPrevPeriod:     new(-14999.50),
				ShortfallRollover:          new(0.0),
				StartDate:                  &p2Start,
			},
			{}, // All nil fields
		}

		list, diags := mapCommitmentPeriods(ctx, &periods)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		var got []datasource_commitment.PeriodsValue
		if d := list.ElementsAs(ctx, &got, false); d.HasError() {
			t.Fatalf("ElementsAs failed: %v", d)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 periods, got %d", len(got))
		}

		// Period 0: populated with shortfall carried forward
		if got[0].CashAttainment.ValueFloat64() != 85000.50 {
			t.Errorf("period[0].cash_attainment = %v, want 85000.50", got[0].CashAttainment.ValueFloat64())
		}
		if got[0].CommitmentValue.ValueFloat64() != 100000.00 {
			t.Errorf("period[0].commitment_value = %v, want 100000.00", got[0].CommitmentValue.ValueFloat64())
		}
		if got[0].ForecastValue.ValueFloat64() != 92000.00 {
			t.Errorf("period[0].forecast_value = %v, want 92000.00", got[0].ForecastValue.ValueFloat64())
		}
		if got[0].MarketplaceLimitAmount.ValueFloat64() != 25000.00 {
			t.Errorf("period[0].marketplace_limit_amount = %v, want 25000.00", got[0].MarketplaceLimitAmount.ValueFloat64())
		}
		if got[0].MarketplaceLimitPercentage.ValueFloat64() != 25.0 {
			t.Errorf("period[0].marketplace_limit_percentage = %v, want 25.0", got[0].MarketplaceLimitPercentage.ValueFloat64())
		}
		if got[0].MarketplaceSpend.ValueFloat64() != 12000.25 {
			t.Errorf("period[0].marketplace_spend = %v, want 12000.25", got[0].MarketplaceSpend.ValueFloat64())
		}
		if got[0].RolloverFromPrevPeriod.ValueFloat64() != 0.0 {
			t.Errorf("period[0].rollover_from_prev_period = %v, want 0.0", got[0].RolloverFromPrevPeriod.ValueFloat64())
		}
		if got[0].ShortfallRollover.ValueFloat64() != 14999.50 {
			t.Errorf("period[0].shortfall_rollover = %v, want 14999.50", got[0].ShortfallRollover.ValueFloat64())
		}
		if got[0].StartDate.ValueString() != "2024-01-01T00:00:00Z" {
			t.Errorf("period[0].start_date = %q, want %q", got[0].StartDate.ValueString(), "2024-01-01T00:00:00Z")
		}
		if got[0].EndDate.ValueString() != "2025-01-01T00:00:00Z" {
			t.Errorf("period[0].end_date = %q, want %q", got[0].EndDate.ValueString(), "2025-01-01T00:00:00Z")
		}

		// Period 1: negative rollover_from_prev_period
		if got[1].CashAttainment.ValueFloat64() != 115000.00 {
			t.Errorf("period[1].cash_attainment = %v, want 115000.00", got[1].CashAttainment.ValueFloat64())
		}
		if got[1].RolloverFromPrevPeriod.ValueFloat64() != -14999.50 {
			t.Errorf("period[1].rollover_from_prev_period = %v, want -14999.50", got[1].RolloverFromPrevPeriod.ValueFloat64())
		}
		if got[1].ShortfallRollover.ValueFloat64() != 0.0 {
			t.Errorf("period[1].shortfall_rollover = %v, want 0.0", got[1].ShortfallRollover.ValueFloat64())
		}

		// Period 2: all nil fields map to null
		if !got[2].CashAttainment.IsNull() {
			t.Errorf("expected period[2].cash_attainment to be null, got %v", got[2].CashAttainment)
		}
		if !got[2].RolloverFromPrevPeriod.IsNull() {
			t.Errorf("expected period[2].rollover_from_prev_period to be null, got %v", got[2].RolloverFromPrevPeriod)
		}
		if !got[2].ShortfallRollover.IsNull() {
			t.Errorf("expected period[2].shortfall_rollover to be null, got %v", got[2].ShortfallRollover)
		}
		if !got[2].CommitmentValue.IsNull() {
			t.Errorf("expected period[2].commitment_value to be null, got %v", got[2].CommitmentValue)
		}
	})
}

func TestCommitmentDataSource_UnknownGuard(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("API should not have been called when id is unknown, received %s %s", r.Method, r.URL)
	}))

	resp := readCommitment(t, server, tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state commitmentDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if !state.Name.IsUnknown() {
		t.Errorf("expected Name to be unknown, got %v", state.Name)
	}
	if !state.TotalCurrentAttainment.IsUnknown() {
		t.Errorf("expected TotalCurrentAttainment to be unknown, got %v", state.TotalCurrentAttainment)
	}
	if !state.Periods.IsUnknown() {
		t.Errorf("expected Periods to be unknown, got %v", state.Periods)
	}
}

func TestCommitmentDataSource_Read(t *testing.T) {
	for _, tc := range []struct {
		name         string
		commitmentID string
		status       int
		body         string
		wantErr      bool
		errPattern   string
	}{
		{
			name:         "successful read with cash attainment and rollover fields",
			commitmentID: "comm-123",
			status:       http.StatusOK,
			body: `{
				"id": "comm-123",
				"name": "GCP 2-Year Enterprise Commitment",
				"cloudProvider": "google-cloud",
				"currency": "USD",
				"createTime": 1704067200000,
				"updateTime": 1704153600000,
				"startDate": "2024-01-01T00:00:00Z",
				"endDate": "2026-01-01T00:00:00Z",
				"totalCommitmentValue": 200000.0,
				"totalCurrentAttainment": 180000.0,
				"totalForecastValue": 205000.0,
				"totalMarketplaceSpend": 30000.0,
				"periods": [
					{
						"startDate": "2024-01-01T00:00:00Z",
						"endDate": "2025-01-01T00:00:00Z",
						"commitmentValue": 100000.0,
						"cashAttainment": 90000.0,
						"rolloverFromPrevPeriod": 0.0,
						"shortfallRollover": 10000.0,
						"forecastValue": 90000.0,
						"marketplaceSpend": 15000.0,
						"marketplaceLimitAmount": 25000.0,
						"marketplaceLimitPercentage": 25.0
					},
					{
						"startDate": "2025-01-01T00:00:00Z",
						"endDate": "2026-01-01T00:00:00Z",
						"commitmentValue": 100000.0,
						"cashAttainment": 90000.0,
						"rolloverFromPrevPeriod": -10000.0,
						"shortfallRollover": 0.0,
						"forecastValue": 115000.0,
						"marketplaceSpend": 15000.0,
						"marketplaceLimitAmount": 25000.0,
						"marketplaceLimitPercentage": 25.0
					}
				]
			}`,
			wantErr: false,
		},
		{
			name:         "not found 404",
			commitmentID: "missing-comm",
			status:       http.StatusNotFound,
			body:         `{"status": 404, "title": "Not Found"}`,
			wantErr:      true,
			errPattern:   "Commitment not found",
		},
		{
			name:         "server error 500",
			commitmentID: "comm-123",
			status:       http.StatusInternalServerError,
			body:         `{"status": 500, "title": "Internal Server Error"}`,
			wantErr:      true,
			errPattern:   "Status: 500",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/analytics/v1/commitment-manager/" + tc.commitmentID
				if r.Method != http.MethodGet || r.URL.Path != expectedPath {
					t.Errorf("unexpected request: %s %s, want GET %s", r.Method, r.URL, expectedPath)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			resp := readCommitment(t, server, tftypes.NewValue(tftypes.String, tc.commitmentID))
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

			var state commitmentDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if state.TotalCurrentAttainment.ValueFloat64() != 180000.0 {
				t.Errorf("total_current_attainment = %v, want 180000.0", state.TotalCurrentAttainment.ValueFloat64())
			}

			var periods []datasource_commitment.PeriodsValue
			if diags := state.Periods.ElementsAs(t.Context(), &periods, false); diags.HasError() {
				t.Fatal(diags)
			}
			if len(periods) != 2 {
				t.Fatalf("expected 2 periods, got %d", len(periods))
			}

			if periods[0].CashAttainment.ValueFloat64() != 90000.0 {
				t.Errorf("periods[0].cash_attainment = %v, want 90000.0", periods[0].CashAttainment.ValueFloat64())
			}
			if periods[0].RolloverFromPrevPeriod.ValueFloat64() != 0.0 {
				t.Errorf("periods[0].rollover_from_prev_period = %v, want 0.0", periods[0].RolloverFromPrevPeriod.ValueFloat64())
			}
			if periods[0].ShortfallRollover.ValueFloat64() != 10000.0 {
				t.Errorf("periods[0].shortfall_rollover = %v, want 10000.0", periods[0].ShortfallRollover.ValueFloat64())
			}

			if periods[1].CashAttainment.ValueFloat64() != 90000.0 {
				t.Errorf("periods[1].cash_attainment = %v, want 90000.0", periods[1].CashAttainment.ValueFloat64())
			}
			if periods[1].RolloverFromPrevPeriod.ValueFloat64() != -10000.0 {
				t.Errorf("periods[1].rollover_from_prev_period = %v, want -10000.0", periods[1].RolloverFromPrevPeriod.ValueFloat64())
			}
			if periods[1].ShortfallRollover.ValueFloat64() != 0.0 {
				t.Errorf("periods[1].shortfall_rollover = %v, want 0.0", periods[1].ShortfallRollover.ValueFloat64())
			}
		})
	}
}

func readCommitment(t *testing.T, server *httptest.Server, id tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewCommitmentDataSource()
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
	values["id"] = id

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
