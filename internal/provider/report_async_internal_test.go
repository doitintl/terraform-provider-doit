package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAsyncReportDataSources(t *testing.T) {
	for _, saved := range []bool{false, true} {
		t.Run(fmt.Sprintf("saved=%v", saved), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()
				polls := 0
				resultBody := `{"schema":[{"name":"cost","type":"float"}],"rows":[["service",12.5,null]],"forecastRows":[[1]],"secondaryRows":[[2]],"cacheHit":false}`
				server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch r.URL.Path {
					case "/analytics/v1/reports/actions/run", "/analytics/v1/reports/report-1/actions/run":
						if r.Method != http.MethodPost || r.Header.Get("Idempotency-Key") == "" {
							t.Errorf("invalid submission: %s %s", r.Method, r.URL)
						}
						if saved {
							if r.URL.Query().Get("timeRange") != "P7D" {
								t.Errorf("missing saved report override: %s", r.URL)
							}
						} else {
							var body models.AsyncRunInlineJSONRequestBody
							if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
								t.Fatal(err)
							}
							if body.Config == nil || body.Config.Layout == nil || string(*body.Config.Layout) != "table" {
								t.Errorf("config not forwarded: %+v", body)
							}
						}
						w.WriteHeader(202)
						fmt.Fprint(w, `{"operationId":"op-1","status":"pending"}`)
					case "/analytics/v1/reports/operations/op-1":
						polls++
						if polls == 1 {
							w.Header().Set("Retry-After", "3")
							fmt.Fprint(w, `{"operationId":"op-1","status":"running"}`)
						} else {
							fmt.Fprint(w, `{"operationId":"op-1","status":"succeeded"}`)
						}
					case "/analytics/v1/reports/operations/op-1/results":
						if polls != 2 {
							t.Error("results requested before success")
						}
						fmt.Fprintf(w, `{"reportName":"Example","result":%s}`, resultBody)
					default:
						t.Errorf("unexpected endpoint %s", r.URL)
						w.WriteHeader(404)
					}
				}))
				client := asyncTestClient(t, server, false)
				var ds datasource.DataSource = &reportQueryDataSource{client: client}
				if saved {
					ds = &reportResultDataSource{client: client}
				}
				resp := readAsyncDataSource(t, ds, saved, false)
				if resp.Diagnostics.HasError() {
					t.Fatal(resp.Diagnostics)
				}
				if time.Since(started) != 3*time.Second {
					t.Fatalf("poll delay = %v", time.Since(started))
				}
				var jsonResult types.String
				var count types.Int64
				var cache types.Bool
				if saved {
					var state reportResultDataSourceModel
					if d := resp.State.Get(t.Context(), &state); d.HasError() {
						t.Fatal(d)
					}
					if state.ReportName.ValueString() != "Example" || state.TimeRange.ValueString() != "P7D" {
						t.Fatalf("bad metadata: %+v", state)
					}
					jsonResult, count, cache = state.ResultJSON, state.RowCount, state.CacheHit
				} else {
					var state reportQueryDataSourceModel
					if d := resp.State.Get(t.Context(), &state); d.HasError() {
						t.Fatal(d)
					}
					jsonResult, count, cache = state.ResultJSON, state.RowCount, state.CacheHit
				}
				var actual, expected any
				if err := json.Unmarshal([]byte(jsonResult.ValueString()), &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(resultBody), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) || count.ValueInt64() != 1 || !cache.Equal(types.BoolValue(false)) {
					t.Fatalf("bad outputs: %s %v %v", jsonResult, count, cache)
				}
			})
		})
	}
}

func TestAsyncReportUnknownInput(t *testing.T) {
	for _, saved := range []bool{false, true} {
		t.Run(fmt.Sprint(saved), func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unknown input made request") }))
			client := asyncTestClient(t, server, false)
			var ds datasource.DataSource = &reportQueryDataSource{client: client}
			if saved {
				ds = &reportResultDataSource{client: client}
			}
			resp := readAsyncDataSource(t, ds, saved, true)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if saved {
				var output reportResultDataSourceModel
				if d := resp.State.Get(t.Context(), &output); d.HasError() {
					t.Fatal(d)
				}
				if !output.ResultJSON.IsUnknown() || !output.ReportName.IsUnknown() {
					t.Fatal("outputs must be unknown")
				}
			} else {
				var output reportQueryDataSourceModel
				if d := resp.State.Get(t.Context(), &output); d.HasError() {
					t.Fatal(d)
				}
				if !output.ResultJSON.IsUnknown() {
					t.Fatal("result must be unknown")
				}
			}
		})
	}
}

func TestAsyncReportErrors(t *testing.T) {
	for _, tc := range []struct {
		name, submit, poll, result, want string
		status                           int
	}{
		{name: "missing ID", submit: `{"status":"pending"}`, want: "operation ID"},
		{name: "failed", poll: `{"operationId":"op-1","status":"failed","error":{"title":"Too large","code":"result_too_large","status":413,"detail":"Reduce the range"}}`, want: "result_too_large"},
		{name: "canceled", poll: `{"operationId":"op-1","status":"canceled"}`, want: "canceled"},
		{name: "unknown status", poll: `{"operationId":"op-1","status":"other"}`, want: "unknown status"},
		{name: "wrong operation", poll: `{"operationId":"other","status":"succeeded"}`, want: "did not match"},
		{name: "expired", status: 404, want: "404"},
		{name: "missing results", result: "null", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/analytics/v1/reports/actions/run":
					w.WriteHeader(202)
					if tc.submit != "" {
						fmt.Fprint(w, tc.submit)
					} else {
						fmt.Fprint(w, `{"operationId":"op-1","status":"pending"}`)
					}
				case "/analytics/v1/reports/operations/op-1":
					if tc.status != 0 {
						w.WriteHeader(tc.status)
						fmt.Fprint(w, `{"error":"not found"}`)
					} else if tc.poll != "" {
						fmt.Fprint(w, tc.poll)
					} else {
						fmt.Fprint(w, `{"operationId":"op-1","status":"succeeded"}`)
					}
				case "/analytics/v1/reports/operations/op-1/results":
					fmt.Fprintf(w, `{"result":%s}`, tc.result)
				default:
					t.Errorf("unexpected request %s", r.URL)
					w.WriteHeader(404)
				}
			}))
			result, d := runAsyncReport(t.Context(), asyncTestClient(t, server, false), &models.ExternalConfig{}, "", nil)
			if tc.want != "" {
				if !d.HasError() || !strings.Contains(fmt.Sprint(d), tc.want) {
					t.Fatalf("got %v, want %q", d, tc.want)
				}
			} else {
				if d.HasError() {
					t.Fatal(d)
				}
				body, cache, count, d := asyncReportOutputs(result.Result)
				if d.HasError() || body.ValueString() != "{}" || !cache.IsNull() || count.ValueInt64() != 0 {
					t.Fatal("bad empty output")
				}
			}
		})
	}
}

func TestAsyncReportTimeoutCancellation(t *testing.T) {
	for _, cancelStatus := range []int{200, 500} {
		t.Run(fmt.Sprint(cancelStatus), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				canceled := false
				server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch r.URL.Path {
					case "/analytics/v1/reports/actions/run":
						w.WriteHeader(202)
						fmt.Fprint(w, `{"operationId":"op-1","status":"pending"}`)
					case "/analytics/v1/reports/operations/op-1":
						w.Header().Set("Retry-After", "10")
						fmt.Fprint(w, `{"operationId":"op-1","status":"running"}`)
					case "/analytics/v1/reports/operations/op-1/actions/cancel":
						canceled = true
						if r.Context().Err() != nil || r.Header.Get("Idempotency-Key") == "" || r.Method != "POST" {
							t.Error("invalid cancellation request")
						}
						w.WriteHeader(cancelStatus)
						fmt.Fprint(w, `{"operationId":"op-1","status":"canceled"}`)
					default:
						t.Errorf("unexpected request %s", r.URL)
					}
				}))
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				_, d := runAsyncReport(ctx, asyncTestClient(t, server, false), &models.ExternalConfig{}, "", nil)
				if !canceled || !d.HasError() || !strings.Contains(fmt.Sprint(d), "deadline exceeded") {
					t.Fatalf("canceled=%v diagnostics=%v", canceled, d)
				}
				if (cancelStatus == 500) != (len(d.Warnings()) == 1) {
					t.Fatalf("cancellation warnings: %v", d)
				}
			})
		})
	}
}

func TestAsyncReportSubmissionRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		key := ""
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/analytics/v1/reports/actions/run":
				calls++
				if key == "" {
					key = r.Header.Get("Idempotency-Key")
				}
				if key == "" || key != r.Header.Get("Idempotency-Key") {
					t.Error("retry changed key")
				}
				if calls == 1 {
					w.Header().Set("Retry-After", "2")
					w.WriteHeader(429)
					return
				}
				w.WriteHeader(202)
				fmt.Fprint(w, `{"operationId":"op-1","status":"succeeded"}`)
			case "/analytics/v1/reports/operations/op-1":
				fmt.Fprint(w, `{"operationId":"op-1","status":"succeeded"}`)
			case "/analytics/v1/reports/operations/op-1/results":
				fmt.Fprint(w, `{"result":{"rows":[],"cacheHit":true}}`)
			default:
				t.Errorf("unexpected %s", r.URL)
			}
		}))
		_, d := runAsyncReport(t.Context(), asyncTestClient(t, server, true), &models.ExternalConfig{}, "", nil)
		if d.HasError() || calls != 2 {
			t.Fatalf("calls=%d diagnostics=%v", calls, d)
		}
	})
}

func asyncTestClient(t *testing.T, s *httptest.Server, retry bool) *models.ClientWithResponses {
	t.Helper()
	var httpClient models.HttpRequestDoer = s.Client()
	if retry {
		httpClient = &DCIRetryClient{client: s.Client()}
	}
	client, err := models.NewClientWithResponses(s.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func readAsyncDataSource(t *testing.T, ds datasource.DataSource, saved, unknown bool) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	var sr datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &sr)
	if d := sr.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Fatal(d)
	}
	ts := map[string]tftypes.Type{}
	vs := map[string]tftypes.Value{}
	for n, a := range sr.Schema.Attributes {
		ts[n] = a.GetType().TerraformType(ctx)
		vs[n] = tftypes.NewValue(ts[n], nil)
	}
	vs["async"] = tftypes.NewValue(tftypes.Bool, true)
	if unknown {
		vs["async"] = tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)
	}
	if saved {
		vs["id"] = tftypes.NewValue(tftypes.String, "report-1")
		vs["time_range"] = tftypes.NewValue(tftypes.String, "P7D")
	} else {
		ct := ts["config"].(tftypes.Object)
		cv := map[string]tftypes.Value{}
		for n, typ := range ct.AttributeTypes {
			cv[n] = tftypes.NewValue(typ, nil)
		}
		cv["layout"] = tftypes.NewValue(tftypes.String, "table")
		vs["config"] = tftypes.NewValue(ct, cv)
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: ts}, vs)}}, &resp)
	return resp
}
