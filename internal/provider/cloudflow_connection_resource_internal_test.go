package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	cloudflowConnectionsPath = "/cloudflow/v1/connections"
	cloudflowMinimalID       = "conn-aws-min"
	cloudflowMinimalPath     = cloudflowConnectionsPath + "/" + cloudflowMinimalID
)

// cloudflowMinimalWith returns the minimal AWS response with the given etag and name.
func cloudflowMinimalWith(etag, name string) string {
	quoted, _ := json.Marshal(etag)
	body := strings.Replace(testCloudflowAwsMinimalJSON, `"etag": "\"etag-min\""`, `"etag": `+string(quoted), 1)
	return strings.Replace(body, `"name": "AWS minimal"`, `"name": "`+name+`"`, 1)
}

func cloudflowTestClients() map[string]func(*testing.T, *httptest.Server) *models.ClientWithResponses {
	return map[string]func(*testing.T, *httptest.Server) *models.ClientWithResponses{
		"plain": newTestAPIClient,
		// Production uses the retry client; a status it turns into an error never
		// reaches the resource's status handling.
		"retry": func(t *testing.T, srv *httptest.Server) *models.ClientWithResponses {
			return newTestRetryAPIClient(t, srv, DefaultRequestTimeout, constantBackOff(time.Millisecond))
		},
	}
}

func cloudflowTestSchema(ctx context.Context, t *testing.T, r resource.Resource) resource.SchemaResponse {
	t.Helper()
	resp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	return resp
}

func cloudflowTestPlan(ctx context.Context, t *testing.T, r resource.Resource, model cloudflowConnectionResourceModel) tfsdk.Plan {
	t.Helper()
	s := cloudflowTestSchema(ctx, t, r).Schema
	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := plan.Set(ctx, &model); diags.HasError() {
		t.Fatalf("set plan: %v", diags)
	}
	return plan
}

func cloudflowTestState(ctx context.Context, t *testing.T, r resource.Resource, model cloudflowConnectionResourceModel) tfsdk.State {
	t.Helper()
	s := cloudflowTestSchema(ctx, t, r).Schema
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	return state
}

func cloudflowEmptyState(ctx context.Context, t *testing.T, r resource.Resource) tfsdk.State {
	t.Helper()
	s := cloudflowTestSchema(ctx, t, r).Schema
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
}

func cloudflowStateModel(ctx context.Context, t *testing.T, state tfsdk.State) cloudflowConnectionResourceModel {
	t.Helper()
	var model cloudflowConnectionResourceModel
	if diags := state.Get(ctx, &model); diags.HasError() {
		t.Fatalf("get state: %v", diags)
	}
	return model
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func TestCloudflowConnectionResourceCreate(t *testing.T) {
	const createdID = cloudflowMinimalID

	tests := []struct {
		name         string
		createStatus int
		createBody   string
		getStatus    int
		wantError    bool
		wantWarnings int
		wantEtag     string
		wantGets     int32
	}{
		{name: "state comes from the follow-up GET", createStatus: 201, createBody: cloudflowMinimalWith("etag-create", "AWS minimal"), getStatus: 200, wantEtag: "etag-get", wantGets: 1},
		{name: "denied follow-up GET falls back to the create response", createStatus: 201, createBody: cloudflowMinimalWith("etag-create", "AWS minimal"), getStatus: 403, wantWarnings: 1, wantEtag: "etag-create", wantGets: 1},
		{name: "follow-up GET not found falls back to the create response", createStatus: 201, createBody: cloudflowMinimalWith("etag-create", "AWS minimal"), getStatus: 404, wantEtag: "etag-create", wantGets: 1},
		{name: "API rejection is reported with status and body", createStatus: 400, createBody: `{"code":"invalid_connection","detail":"role already exists"}`, wantError: true},
		{name: "created connection without an id", createStatus: 201, createBody: `{"name":"x"}`, wantError: true},
	}

	for clientName, newClient := range cloudflowTestClients() {
		for _, tt := range tests {
			t.Run(clientName+"/"+tt.name, func(t *testing.T) {
				var gets, posts atomic.Int32
				var gotKey, gotBody string
				srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodPost && r.URL.Path == cloudflowConnectionsPath:
						posts.Add(1)
						gotKey = r.Header.Get("Idempotency-Key")
						raw, _ := io.ReadAll(r.Body)
						gotBody = string(raw)
						writeJSON(w, tt.createStatus, tt.createBody)
					case r.Method == http.MethodGet && r.URL.Path == cloudflowMinimalPath:
						gets.Add(1)
						if tt.getStatus != 200 {
							writeJSON(w, tt.getStatus, `{"code":"forbidden"}`)
							return
						}
						writeJSON(w, 200, cloudflowMinimalWith("etag-get", "AWS minimal"))
					default:
						w.WriteHeader(http.StatusInternalServerError)
					}
				}))

				ctx, cancel := context.WithTimeout(t.Context(), retryTestTimeout)
				defer cancel()
				r := &cloudflowConnectionResource{client: newClient(t, srv)}

				plan := mapCloudflowConnectionForTest(t, cloudflowMinimalWith("etag-min", "AWS minimal"))
				cloudflowConnectionUnknownComputed(t, &plan)
				resp := &resource.CreateResponse{State: cloudflowEmptyState(ctx, t, r)}
				r.Create(ctx, resource.CreateRequest{Plan: cloudflowTestPlan(ctx, t, r, plan)}, resp)

				if got := resp.Diagnostics.HasError(); got != tt.wantError {
					t.Fatalf("hasError = %v, want %v: %v", got, tt.wantError, resp.Diagnostics)
				}
				if got := posts.Load(); got != 1 {
					t.Errorf("POST calls = %d, want 1", got)
				}
				if gotKey == "" {
					t.Errorf("create sent no Idempotency-Key")
				}
				if tt.wantError {
					if tt.createStatus != 201 && !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "400") {
						t.Errorf("error does not carry the status: %v", resp.Diagnostics)
					}
					if tt.createStatus != 201 && !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "role already exists") {
						t.Errorf("error does not carry the body: %v", resp.Diagnostics)
					}
					return
				}

				if got := gets.Load(); got != tt.wantGets {
					t.Errorf("GET calls = %d, want %d", got, tt.wantGets)
				}
				if got := resp.Diagnostics.WarningsCount(); got != tt.wantWarnings {
					t.Errorf("warnings = %d, want %d: %v", got, tt.wantWarnings, resp.Diagnostics)
				}

				var sent map[string]any
				if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
					t.Fatalf("request body: %v", err)
				}
				if sent["name"] != "AWS minimal" || sent["awsConfig"] == nil || sent["gcpConfig"] != nil {
					t.Errorf("request body = %s", gotBody)
				}

				state := cloudflowStateModel(ctx, t, resp.State)
				if state.ConnectionId.ValueString() != createdID || state.Etag.ValueString() != tt.wantEtag {
					t.Errorf("state id/etag = %q %q, want %q %q", state.ConnectionId.ValueString(), state.Etag.ValueString(), createdID, tt.wantEtag)
				}
				if state.Etag.IsUnknown() || state.UpdatedAt.IsUnknown() || state.Status.IsUnknown() {
					t.Errorf("unknown computed value left in state")
				}
			})
		}
	}
}

func TestCloudflowConnectionResourceCreate_FreshIdempotencyKeys(t *testing.T) {
	var keys []string
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			keys = append(keys, r.Header.Get("Idempotency-Key"))
			writeJSON(w, 201, cloudflowMinimalWith("e", "AWS minimal"))
			return
		}
		writeJSON(w, 200, cloudflowMinimalWith("e", "AWS minimal"))
	}))

	ctx := t.Context()
	r := &cloudflowConnectionResource{client: newTestAPIClient(t, srv)}
	for range 2 {
		plan := mapCloudflowConnectionForTest(t, testCloudflowAwsMinimalJSON)
		cloudflowConnectionUnknownComputed(t, &plan)
		resp := &resource.CreateResponse{State: cloudflowEmptyState(ctx, t, r)}
		r.Create(ctx, resource.CreateRequest{Plan: cloudflowTestPlan(ctx, t, r, plan)}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("create: %v", resp.Diagnostics)
		}
	}
	if len(keys) != 2 || keys[0] == "" || keys[0] == keys[1] {
		t.Errorf("idempotency keys = %q, want two distinct non-empty keys", keys)
	}
}

func TestCloudflowConnectionResourceUpdate(t *testing.T) {
	tests := []struct {
		name         string
		stateEtag    string
		currentEtag  string // the etag the server accepts; GET returns it until the PATCH succeeds
		patchFinal   int    // status once the etag matches
		wantPatches  int32
		wantGets     int32 // includes the follow-up GET after a successful PATCH
		wantError    bool
		wantConflict bool
	}{
		{name: "etag from state", stateEtag: "e1", currentEtag: "e1", patchFinal: 200, wantPatches: 1, wantGets: 1},
		{name: "stale etag is a conflict, not retried", stateEtag: "old", currentEtag: "e2", patchFinal: 200, wantPatches: 1, wantGets: 0, wantError: true, wantConflict: true},
		{name: "no etag in state fetched first", stateEtag: "", currentEtag: "e3", patchFinal: 200, wantPatches: 1, wantGets: 2},
		{name: "validation error carries status and body", stateEtag: "e1", currentEtag: "e1", patchFinal: 400, wantPatches: 1, wantGets: 0, wantError: true},
		{name: "forbidden carries status and body", stateEtag: "e1", currentEtag: "e1", patchFinal: 403, wantPatches: 1, wantGets: 0, wantError: true},
	}

	for clientName, newClient := range cloudflowTestClients() {
		for _, tt := range tests {
			t.Run(clientName+"/"+tt.name, func(t *testing.T) {
				var patches, gets atomic.Int32
				var patched atomic.Bool
				var gotBody string
				srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != cloudflowMinimalPath {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					switch r.Method {
					case http.MethodGet:
						gets.Add(1)
						if patched.Load() {
							writeJSON(w, 200, cloudflowMinimalWith("after-patch", "renamed"))
							return
						}
						writeJSON(w, 200, cloudflowMinimalWith(tt.currentEtag, "AWS minimal"))
					case http.MethodPatch:
						patches.Add(1)
						if r.Header.Get("If-Match") != tt.currentEtag {
							w.WriteHeader(http.StatusPreconditionFailed)
							return
						}
						if tt.patchFinal != 200 {
							writeJSON(w, tt.patchFinal, `{"code":"invalid_connection","detail":"bad config"}`)
							return
						}
						raw, _ := io.ReadAll(r.Body)
						gotBody = string(raw)
						patched.Store(true)
						writeJSON(w, 200, cloudflowMinimalWith("patch-response", "renamed"))
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
				}))

				ctx, cancel := context.WithTimeout(t.Context(), retryTestTimeout)
				defer cancel()
				r := &cloudflowConnectionResource{client: newClient(t, srv)}

				state := mapCloudflowConnectionForTest(t, cloudflowMinimalWith("old-etag", "AWS minimal"))
				state.Etag = types.StringNull()
				if tt.stateEtag != "" {
					state.Etag = types.StringValue(tt.stateEtag)
				}

				// The plan: renamed, with the volatile computed fields unknown and the stable ones kept.
				plan := state
				plan.Name = types.StringValue("renamed")
				cloudflowConnectionUnknownComputed(t, &plan)
				plan.ConnectionId = state.ConnectionId
				plan.CreatedAt = state.CreatedAt

				resp := &resource.UpdateResponse{State: cloudflowTestState(ctx, t, r, state)}
				r.Update(ctx, resource.UpdateRequest{
					Plan:  cloudflowTestPlan(ctx, t, r, plan),
					State: cloudflowTestState(ctx, t, r, state),
				}, resp)

				if got := resp.Diagnostics.HasError(); got != tt.wantError {
					t.Fatalf("hasError = %v, want %v: %v", got, tt.wantError, resp.Diagnostics)
				}
				if got := patches.Load(); got != tt.wantPatches {
					t.Errorf("PATCH calls = %d, want %d", got, tt.wantPatches)
				}
				if got := gets.Load(); got != tt.wantGets {
					t.Errorf("GET calls = %d, want %d", got, tt.wantGets)
				}
				if tt.wantConflict {
					if got := resp.Diagnostics.Errors()[0].Summary(); got != "CloudFlow Connection Changed Outside Terraform" {
						t.Errorf("error summary = %q, want the conflict error", got)
					}
					return
				}
				if tt.wantError {
					detail := resp.Diagnostics.Errors()[0].Detail()
					if !strings.Contains(detail, strconv.Itoa(tt.patchFinal)) || !strings.Contains(detail, "bad config") {
						t.Errorf("error %q lacks the status %d or the body", detail, tt.patchFinal)
					}
					return
				}

				var sent map[string]any
				if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
					t.Fatalf("request body: %v", err)
				}
				if sent["name"] != "renamed" || sent["awsConfig"] == nil {
					t.Errorf("request body = %s", gotBody)
				}

				got := cloudflowStateModel(ctx, t, resp.State)
				if got.Name.ValueString() != "renamed" || got.Etag.ValueString() != "after-patch" {
					t.Errorf("state name/etag = %q %q, want renamed and the follow-up GET's etag", got.Name.ValueString(), got.Etag.ValueString())
				}
				if got.Etag.IsUnknown() || got.UpdatedAt.IsUnknown() || got.Status.IsUnknown() {
					t.Errorf("unknown computed value left in state")
				}
			})
		}
	}
}

func TestCloudflowConnectionResourceDelete(t *testing.T) {
	tests := []struct {
		name        string
		stateEtag   string
		currentEtag string // etag the server accepts; GET returns it
		getStatus   int
		deleteFinal int // status once the etag matches
		wantDeletes int32
		wantGets    int32
		wantError   bool
		wantInError string
	}{
		{name: "etag from state", stateEtag: "e1", currentEtag: "e1", getStatus: 200, deleteFinal: 204, wantDeletes: 1, wantGets: 0},
		{name: "stale etag retried once", stateEtag: "old", currentEtag: "e2", getStatus: 200, deleteFinal: 204, wantDeletes: 2, wantGets: 1},
		{name: "no etag in state fetched first", stateEtag: "", currentEtag: "e3", getStatus: 200, deleteFinal: 204, wantDeletes: 1, wantGets: 1},
		{name: "already gone", stateEtag: "e1", currentEtag: "e1", getStatus: 404, deleteFinal: 404, wantDeletes: 1, wantGets: 0},
		{name: "gone before retry", stateEtag: "old", currentEtag: "e2", getStatus: 404, deleteFinal: 204, wantDeletes: 1, wantGets: 1},
		{name: "in use by flows", stateEtag: "e1", currentEtag: "e1", getStatus: 200, deleteFinal: 409, wantDeletes: 1, wantGets: 0, wantError: true, wantInError: "in use by one or more flows"},
		{name: "server error", stateEtag: "e1", currentEtag: "e1", getStatus: 200, deleteFinal: 500, wantDeletes: 1, wantGets: 0, wantError: true, wantInError: "500"},
		{name: "precondition required is surfaced", stateEtag: "e1", currentEtag: "e1", getStatus: 200, deleteFinal: 428, wantDeletes: 1, wantGets: 0, wantError: true, wantInError: "428"},
	}

	for clientName, newClient := range cloudflowTestClients() {
		for _, tt := range tests {
			t.Run(clientName+"/"+tt.name, func(t *testing.T) {
				var deletes, gets atomic.Int32
				srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != cloudflowMinimalPath {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					switch r.Method {
					case http.MethodGet:
						gets.Add(1)
						if tt.getStatus != 200 {
							writeJSON(w, tt.getStatus, `{"code":"not_found"}`)
							return
						}
						writeJSON(w, 200, cloudflowMinimalWith(tt.currentEtag, "AWS minimal"))
					case http.MethodDelete:
						deletes.Add(1)
						if r.Header.Get("If-Match") != tt.currentEtag {
							w.WriteHeader(http.StatusPreconditionFailed)
							return
						}
						if tt.deleteFinal == 204 {
							w.WriteHeader(http.StatusNoContent)
							return
						}
						writeJSON(w, tt.deleteFinal, `{"code":"x"}`)
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
				}))

				ctx, cancel := context.WithTimeout(t.Context(), retryTestTimeout)
				defer cancel()
				r := &cloudflowConnectionResource{client: newClient(t, srv)}

				state := mapCloudflowConnectionForTest(t, cloudflowMinimalWith("old", "AWS minimal"))
				state.Etag = types.StringNull()
				if tt.stateEtag != "" {
					state.Etag = types.StringValue(tt.stateEtag)
				}

				resp := &resource.DeleteResponse{}
				r.Delete(ctx, resource.DeleteRequest{State: cloudflowTestState(ctx, t, r, state)}, resp)

				if got := resp.Diagnostics.HasError(); got != tt.wantError {
					t.Errorf("hasError = %v, want %v: %v", got, tt.wantError, resp.Diagnostics)
				}
				if tt.wantError && !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tt.wantInError) {
					t.Errorf("error %q does not contain %q", resp.Diagnostics.Errors()[0].Detail(), tt.wantInError)
				}
				if got := deletes.Load(); got != tt.wantDeletes {
					t.Errorf("DELETE calls = %d, want %d", got, tt.wantDeletes)
				}
				if got := gets.Load(); got != tt.wantGets {
					t.Errorf("GET calls = %d, want %d", got, tt.wantGets)
				}
			})
		}
	}
}

func TestCloudflowConnectionResourceRead(t *testing.T) {
	t.Run("maps the connection", func(t *testing.T) {
		srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, testCloudflowGcpJSON)
		}))
		ctx := t.Context()
		r := &cloudflowConnectionResource{client: newTestAPIClient(t, srv)}
		prior := mapCloudflowConnectionForTest(t, testCloudflowGcpMinimalJSON)
		prior.ConnectionId = types.StringValue("conn-gcp")

		resp := &resource.ReadResponse{State: cloudflowTestState(ctx, t, r, prior)}
		r.Read(ctx, resource.ReadRequest{State: cloudflowTestState(ctx, t, r, prior)}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("read: %v", resp.Diagnostics)
		}
		got := cloudflowStateModel(ctx, t, resp.State)
		if got.Name.ValueString() != "GCP project" || got.GcpConfig.ProjectId.ValueString() != "my-project" || got.Etag.ValueString() != `"etag-gcp"` {
			t.Errorf("state = %q %q %q", got.Name.ValueString(), got.GcpConfig.ProjectId.ValueString(), got.Etag.ValueString())
		}
	})

	t.Run("a deleted connection is removed from state", func(t *testing.T) {
		srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 404, `{"code":"connection_not_found"}`)
		}))
		ctx := t.Context()
		r := &cloudflowConnectionResource{client: newTestAPIClient(t, srv)}
		prior := mapCloudflowConnectionForTest(t, testCloudflowGcpMinimalJSON)

		resp := &resource.ReadResponse{State: cloudflowTestState(ctx, t, r, prior)}
		r.Read(ctx, resource.ReadRequest{State: cloudflowTestState(ctx, t, r, prior)}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("read should not fail on 404: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Errorf("state not removed on 404")
		}
	})

	t.Run("an unexpected status is an error with status and body", func(t *testing.T) {
		srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 403, `{"code":"forbidden","detail":"no access"}`)
		}))
		ctx := t.Context()
		r := &cloudflowConnectionResource{client: newTestAPIClient(t, srv)}
		prior := mapCloudflowConnectionForTest(t, testCloudflowGcpMinimalJSON)

		resp := &resource.ReadResponse{State: cloudflowTestState(ctx, t, r, prior)}
		r.Read(ctx, resource.ReadRequest{State: cloudflowTestState(ctx, t, r, prior)}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error")
		}
		detail := resp.Diagnostics.Errors()[0].Detail()
		if !strings.Contains(detail, "403") || !strings.Contains(detail, "no access") {
			t.Errorf("error %q lacks status or body", detail)
		}
	})
}

// Every update changes updated_at, etag and status on the server, including one that
// only clears a value: the plan modifiers that clear it run after the framework has
// decided which computed values to mark unknown.
func TestCloudflowConnectionModifyPlanMarksServerFieldsUnknownOnChange(t *testing.T) {
	ctx := t.Context()
	r := &cloudflowConnectionResource{}

	state := mapCloudflowConnectionForTest(t, cloudflowMinimalWith("e1", "AWS minimal"))
	state.Description = types.StringValue("to be cleared")

	tests := []struct {
		name        string
		edit        func(*cloudflowConnectionResourceModel)
		wantUnknown bool
	}{
		{"no change", func(*cloudflowConnectionResourceModel) {}, false},
		{"description cleared", func(m *cloudflowConnectionResourceModel) { m.Description = types.StringValue("") }, true},
		{"name changed", func(m *cloudflowConnectionResourceModel) { m.Name = types.StringValue("renamed") }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := state
			tt.edit(&plan)

			resp := &resource.ModifyPlanResponse{Plan: cloudflowTestPlan(ctx, t, r, plan)}
			r.ModifyPlan(ctx, resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: resp.Plan.Schema, Raw: resp.Plan.Raw},
				Plan:   cloudflowTestPlan(ctx, t, r, plan),
				State:  cloudflowTestState(ctx, t, r, state),
			}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("ModifyPlan: %v", resp.Diagnostics)
			}

			var got cloudflowConnectionResourceModel
			if diags := resp.Plan.Get(ctx, &got); diags.HasError() {
				t.Fatalf("get plan: %v", diags)
			}
			for name, v := range map[string]interface{ IsUnknown() bool }{"etag": got.Etag, "updated_at": got.UpdatedAt, "status": got.Status} {
				if v.IsUnknown() != tt.wantUnknown {
					t.Errorf("%s unknown = %v, want %v", name, v.IsUnknown(), tt.wantUnknown)
				}
			}
		})
	}
}
