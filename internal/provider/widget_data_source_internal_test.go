package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oapi-codegen/nullable"
)

func TestWidgetDataSourceSchema_ValidateImplementation(t *testing.T) {
	ctx := t.Context()
	ds := NewWidgetDataSource()

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("Widget schema validation failed: %v", diags)
	}
}

func TestWidgetDataSource_UnknownGuard(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("API should not have been called when widget_id is unknown, received %s %s", r.Method, r.URL)
	}))

	resp := readWidget(t, server, tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", resp.Diagnostics)
	}

	var state widgetDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if !state.Alias.IsUnknown() {
		t.Errorf("expected Alias to be unknown, got %v", state.Alias)
	}
	if !state.GenerateTime.IsUnknown() {
		t.Errorf("expected GenerateTime to be unknown, got %v", state.GenerateTime)
	}
	if !state.Id.IsUnknown() {
		t.Errorf("expected Id to be unknown, got %v", state.Id)
	}
	if !state.Kind.IsUnknown() {
		t.Errorf("expected Kind to be unknown, got %v", state.Kind)
	}
	if !state.Result.IsUnknown() {
		t.Errorf("expected Result to be unknown, got %v", state.Result)
	}
	if !state.Type.IsUnknown() {
		t.Errorf("expected Type to be unknown, got %v", state.Type)
	}
}

func TestWidgetDataSource_Mapping(t *testing.T) {
	ctx := t.Context()
	now := time.Date(2026, 9, 25, 8, 30, 0, 0, time.UTC)
	periodStart := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	t.Run("fully populated with growth metrics", func(t *testing.T) {
		growthAmt := models.Money{
			Amount:   "123.45",
			Currency: "USD",
		}
		var growthAmtNullable nullable.Nullable[models.Money]
		growthAmtNullable.Set(growthAmt)

		growthPct := 12.34
		var growthPctNullable nullable.Nullable[float64]
		growthPctNullable.Set(growthPct)

		apiWidget := &models.Widget{
			Id:           "test-widget-id",
			Alias:        "current-month-cloud-spend",
			Type:         models.WidgetTypePreset,
			Kind:         models.WidgetKindMonetaryMetric,
			GenerateTime: now,
			Result: models.WidgetResult{
				MonetaryMetric: models.WidgetMonetaryMetric{
					Value: models.Money{
						Amount:   "1000.00",
						Currency: "USD",
					},
					Period: models.WidgetPeriod{
						StartTime: periodStart,
						EndTime:   periodEnd,
					},
					MonthToMonthGrowthAmount:     growthAmtNullable,
					MonthToMonthGrowthPercentage: growthPctNullable,
				},
			},
		}

		var data widgetDataSourceModel
		diags := mapWidgetToModel(ctx, apiWidget, &data)
		if diags.HasError() {
			t.Fatalf("mapWidgetToModel failed: %v", diags)
		}

		if data.Id.ValueString() != "test-widget-id" {
			t.Errorf("unexpected Id: %s", data.Id.ValueString())
		}
		if data.Alias.ValueString() != "current-month-cloud-spend" {
			t.Errorf("unexpected Alias: %s", data.Alias.ValueString())
		}
		if data.Type.ValueString() != "preset" {
			t.Errorf("unexpected Type: %s", data.Type.ValueString())
		}
		if data.Kind.ValueString() != "monetary_metric" {
			t.Errorf("unexpected Kind: %s", data.Kind.ValueString())
		}

		metric := data.Result.MonetaryMetric
		if metric.Value.Amount.ValueString() != "1000.00" {
			t.Errorf("unexpected amount: %s", metric.Value.Amount.ValueString())
		}
		if metric.Value.Currency.ValueString() != "USD" {
			t.Errorf("unexpected currency: %s", metric.Value.Currency.ValueString())
		}
		if metric.Period.StartTime.ValueString() != periodStart.Format(time.RFC3339) {
			t.Errorf("unexpected period start: %s", metric.Period.StartTime.ValueString())
		}
		if metric.Period.EndTime.ValueString() != periodEnd.Format(time.RFC3339) {
			t.Errorf("unexpected period end: %s", metric.Period.EndTime.ValueString())
		}
		if metric.MonthToMonthGrowthPercentage.IsNull() || metric.MonthToMonthGrowthPercentage.ValueFloat64() != 12.34 {
			t.Errorf("unexpected growth percentage: %v", metric.MonthToMonthGrowthPercentage)
		}
		if metric.MonthToMonthGrowthAmount.IsNull() || metric.MonthToMonthGrowthAmount.Amount.ValueString() != "123.45" {
			t.Errorf("unexpected growth amount: %v", metric.MonthToMonthGrowthAmount)
		}
	})

	t.Run("null growth metrics when no prior period comparison exists", func(t *testing.T) {
		apiWidget := &models.Widget{
			Id:           "test-forecast-widget",
			Alias:        "current-month-cloud-forecast",
			Type:         models.WidgetTypePreset,
			Kind:         models.WidgetKindMonetaryMetric,
			GenerateTime: now,
			Result: models.WidgetResult{
				MonetaryMetric: models.WidgetMonetaryMetric{
					Value: models.Money{
						Amount:   "2500.00",
						Currency: "USD",
					},
					Period: models.WidgetPeriod{
						StartTime: periodStart,
						EndTime:   periodEnd,
					},
					MonthToMonthGrowthAmount:     nullable.NewNullNullable[models.Money](),
					MonthToMonthGrowthPercentage: nullable.NewNullNullable[float64](),
				},
			},
		}

		var data widgetDataSourceModel
		diags := mapWidgetToModel(ctx, apiWidget, &data)
		if diags.HasError() {
			t.Fatalf("mapWidgetToModel failed: %v", diags)
		}

		metric := data.Result.MonetaryMetric
		if !metric.MonthToMonthGrowthPercentage.IsNull() {
			t.Errorf("expected null growth percentage, got %v", metric.MonthToMonthGrowthPercentage)
		}
		if !metric.MonthToMonthGrowthAmount.IsNull() {
			t.Errorf("expected null growth amount, got %v", metric.MonthToMonthGrowthAmount)
		}
	})
}

func TestWidgetDataSource_Read(t *testing.T) {
	for _, tc := range []struct {
		name       string
		widgetID   string
		status     int
		body       string
		wantErr    bool
		errPattern string
	}{
		{
			name:     "successful read",
			widgetID: "current-month-cloud-spend",
			status:   http.StatusOK,
			body: `{
				"id": "spend-widget-123",
				"alias": "current-month-cloud-spend",
				"type": "preset",
				"kind": "monetary_metric",
				"generateTime": "2026-09-24T19:49:38.974Z",
				"result": {
					"monetaryMetric": {
						"value": {
							"amount": "4520.50",
							"currency": "USD"
						},
						"period": {
							"startTime": "2026-09-01T00:00:00Z",
							"endTime": "2026-10-01T00:00:00Z"
						},
						"monthToMonthGrowthPercentage": 5.25,
						"monthToMonthGrowthAmount": {
							"amount": "225.50",
							"currency": "USD"
						}
					}
				}
			}`,
			wantErr: false,
		},
		{
			name:       "not found 404 with problem details",
			widgetID:   "nonexistent-widget",
			status:     http.StatusNotFound,
			body:       `{"status": 404, "code": "not_found", "title": "Resource not found", "detail": "no widget exists with that id"}`,
			wantErr:    true,
			errPattern: "Widget Not Found",
		},
		{
			name:       "widget data not available 404",
			widgetID:   "current-month-cloud-spend",
			status:     http.StatusNotFound,
			body:       `{"status": 404, "code": "analytics.widget_data_not_available", "title": "Widget data not available", "detail": "no data has been computed for this customer yet"}`,
			wantErr:    true,
			errPattern: "Widget Data Not Available",
		},
		{
			name:       "widget no billing data 404",
			widgetID:   "current-month-cloud-spend",
			status:     http.StatusNotFound,
			body:       `{"status": 404, "code": "analytics.widget_no_billing_data", "title": "No billing data", "detail": "no billing data is available for this customer"}`,
			wantErr:    true,
			errPattern: "No Billing Data Available",
		},
		{
			name:       "widget generic 404 fallback",
			widgetID:   "current-month-cloud-spend",
			status:     http.StatusNotFound,
			body:       `{"status": 404, "title": "Not Found"}`,
			wantErr:    true,
			errPattern: "Widget Not Found",
		},
		{
			name:       "widget unknown code 404",
			widgetID:   "current-month-cloud-spend",
			status:     http.StatusNotFound,
			body:       `{"status": 404, "code": "analytics.unknown_code", "title": "Unknown", "detail": "something else"}`,
			wantErr:    true,
			errPattern: "Widget Unavailable",
		},
		{
			name:       "server error 500",
			widgetID:   "current-month-cloud-spend",
			status:     http.StatusInternalServerError,
			body:       `{"status": 500, "title": "Internal Server Error"}`,
			wantErr:    true,
			errPattern: "status 500",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/analytics/v1/widgets/" + tc.widgetID
				if r.Method != http.MethodGet || r.URL.Path != expectedPath {
					t.Errorf("unexpected request: %s %s, want GET %s", r.Method, r.URL, expectedPath)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			resp := readWidget(t, server, tftypes.NewValue(tftypes.String, tc.widgetID))
			if tc.wantErr {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected error containing %q, got none", tc.errPattern)
				}
				found := false
				for _, diag := range resp.Diagnostics.Errors() {
					if diag.Summary() == tc.errPattern || strings.Contains(diag.Detail(), tc.errPattern) {
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

			var state widgetDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if state.Id.ValueString() != "spend-widget-123" {
				t.Errorf("unexpected id: %s", state.Id.ValueString())
			}
			if state.Alias.ValueString() != "current-month-cloud-spend" {
				t.Errorf("unexpected alias: %s", state.Alias.ValueString())
			}
			if state.Result.MonetaryMetric.Value.Amount.ValueString() != "4520.50" {
				t.Errorf("unexpected amount: %s", state.Result.MonetaryMetric.Value.Amount.ValueString())
			}
		})
	}
}

func readWidget(t *testing.T, server *httptest.Server, widgetID tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewWidgetDataSource()
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
	values["widget_id"] = widgetID

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
