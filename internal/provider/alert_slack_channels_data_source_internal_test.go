package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_alert_slack_channels"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func readAlertSlackChannelsHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (alertSlackChannelsDataSourceModel, tfsdk.State, diag.Diagnostics) {
	t.Helper()

	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return readAlertSlackChannelsWithClient(t, client, overrides)
}

func readAlertSlackChannelsWithClient(t *testing.T, client *models.ClientWithResponses, overrides map[string]tftypes.Value) (alertSlackChannelsDataSourceModel, tfsdk.State, diag.Diagnostics) {
	t.Helper()

	d := &alertSlackChannelsDataSource{client: client}
	ctx := t.Context()

	schemaResp := &datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}

	configValues := map[string]tftypes.Value{}
	attrTypes := map[string]tftypes.Type{}
	for name, attr := range schemaResp.Schema.Attributes {
		tfType := attr.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if override, ok := overrides[name]; ok {
			configValues[name] = override
			continue
		}
		configValues[name] = tftypes.NewValue(tfType, nil)
	}

	configVal := tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, configValues)
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    configVal,
	}

	readResp := &datasource.ReadResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	d.Read(ctx, datasource.ReadRequest{Config: config}, readResp)

	var data alertSlackChannelsDataSourceModel
	if !readResp.Diagnostics.HasError() {
		if diags := readResp.State.Get(ctx, &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
	}

	return data, readResp.State, readResp.Diagnostics
}

func TestAlertSlackChannelsDataSource_UnknownInputGuard(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		overrides map[string]tftypes.Value
	}{
		{
			name: "unknown name_contains",
			overrides: map[string]tftypes.Value{
				"name_contains": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown max_results",
			overrides: map[string]tftypes.Value{
				"max_results": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown page_token",
			overrides: map[string]tftypes.Value{
				"page_token": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reqCount atomic.Int32
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqCount.Add(1)
				w.WriteHeader(http.StatusOK)
			}))

			data, _, diags := readAlertSlackChannelsHelper(t, server, tc.overrides)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if reqCount.Load() != 0 {
				t.Errorf("expected 0 HTTP requests when config has unknown attributes, got %d", reqCount.Load())
			}

			if !data.Items.IsUnknown() {
				t.Errorf("expected Items to be unknown, got %v", data.Items)
			}
			if !data.RowCount.IsUnknown() {
				t.Errorf("expected RowCount to be unknown, got %v", data.RowCount)
			}
			if !data.WorkspaceStatus.IsUnknown() {
				t.Errorf("expected WorkspaceStatus to be unknown, got %v", data.WorkspaceStatus)
			}
			if !data.IsWorkspaceConnected.IsUnknown() {
				t.Errorf("expected IsWorkspaceConnected to be unknown, got %v", data.IsWorkspaceConnected)
			}
			if !data.HasSharedChannel.IsUnknown() {
				t.Errorf("expected HasSharedChannel to be unknown, got %v", data.HasSharedChannel)
			}
			if !data.PageToken.IsUnknown() {
				t.Errorf("expected PageToken to be unknown, got %v", data.PageToken)
			}
			if !data.ChannelsComplete.IsUnknown() {
				t.Errorf("expected ChannelsComplete to be unknown, got %v", data.ChannelsComplete)
			}
		})
	}
}

func TestAlertSlackChannelsDataSource_Read_AutoPagination(t *testing.T) {
	t.Parallel()

	var reqCount atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := reqCount.Add(1)
		if maxResults := r.URL.Query().Get("maxResults"); maxResults != "500" {
			t.Errorf("request %d: expected auto mode to request maxResults=500, got %q", count, maxResults)
		}
		wantToken := ""
		if count > 1 {
			wantToken = "token-page-2"
		}
		if got := r.URL.Query().Get("pageToken"); got != wantToken {
			t.Errorf("request %d: expected pageToken %q, got %q", count, wantToken, got)
		}
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			token := "token-page-2"
			resp := models.AlertSlackChannelList{
				HasSharedChannel:     true,
				ChannelsComplete:     true,
				IsWorkspaceConnected: true,
				RowCount:             new(3),
				WorkspaceStatus:      "ok",
				PageToken:            &token,
				Items: []models.AlertSlackChannel{
					{
						Id:        "C001",
						Name:      new("general"),
						Workspace: new("my-ws"),
						Shared:    new(false),
						Type:      new(models.AlertSlackChannelType("public")),
					},
					{
						Id:        "C002",
						Name:      new("random"),
						Workspace: new("my-ws"),
						Shared:    nil,
						Type:      new(models.AlertSlackChannelType("public")),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := models.AlertSlackChannelList{
			HasSharedChannel:     true,
			ChannelsComplete:     true,
			IsWorkspaceConnected: true,
			RowCount:             new(3),
			WorkspaceStatus:      "ok",
			PageToken:            nil,
			Items: []models.AlertSlackChannel{
				{
					Id:        "C003",
					Name:      new("shared-alerts"),
					Workspace: new(""),
					Shared:    new(true),
					Type:      new(models.AlertSlackChannelType("private")),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	data, _, diags := readAlertSlackChannelsHelper(t, server, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if reqCount.Load() != 2 {
		t.Errorf("expected 2 HTTP requests for auto-pagination, got %d", reqCount.Load())
	}

	if data.RowCount.ValueInt64() != 3 {
		t.Errorf("expected RowCount 3, got %d", data.RowCount.ValueInt64())
	}

	if !data.PageToken.IsNull() {
		t.Errorf("expected PageToken to be null in auto-pagination mode, got %v", data.PageToken)
	}

	if data.WorkspaceStatus.ValueString() != "ok" {
		t.Errorf("expected WorkspaceStatus 'ok', got %q", data.WorkspaceStatus.ValueString())
	}
	if !data.IsWorkspaceConnected.ValueBool() {
		t.Errorf("expected IsWorkspaceConnected to be true")
	}
	if !data.HasSharedChannel.ValueBool() {
		t.Errorf("expected HasSharedChannel to be true")
	}
	if !data.ChannelsComplete.ValueBool() {
		t.Errorf("expected ChannelsComplete to be true")
	}
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for a complete list, got %v", diags)
	}

	var items []datasource_alert_slack_channels.ItemsValue
	diags = data.Items.ElementsAs(t.Context(), &items, false)
	if diags.HasError() {
		t.Fatalf("failed to decode items: %v", diags)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Verify normalization on item 1 (shared nil -> false)
	if items[1].Shared.IsNull() || items[1].Shared.ValueBool() != false {
		t.Errorf("expected item 1 Shared to normalize nil to false, got %v", items[1].Shared)
	}

	// Verify normalization on item 2 (workspace "" -> null)
	if !items[2].Workspace.IsNull() {
		t.Errorf("expected item 2 Workspace to normalize empty string to null, got %v", items[2].Workspace)
	}
}

func TestAlertSlackChannelsDataSource_Read_ManualPagination(t *testing.T) {
	t.Parallel()

	var reqCount atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)

		if maxResults := r.URL.Query().Get("maxResults"); maxResults != "2" {
			t.Errorf("expected maxResults query param '2', got %q", maxResults)
		}

		w.Header().Set("Content-Type", "application/json")
		token := "next-page-token"
		resp := models.AlertSlackChannelList{
			HasSharedChannel:     true,
			ChannelsComplete:     true,
			IsWorkspaceConnected: true,
			RowCount:             new(10),
			WorkspaceStatus:      "ok",
			PageToken:            &token,
			Items: []models.AlertSlackChannel{
				{
					Id:        "C100",
					Name:      new("dev"),
					Workspace: new("my-ws"),
					Shared:    new(false),
					Type:      new(models.AlertSlackChannelType("public")),
				},
				{
					Id:        "C101",
					Name:      new("prod"),
					Workspace: new("my-ws"),
					Shared:    new(false),
					Type:      new(models.AlertSlackChannelType("public")),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	overrides := map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, 2),
	}

	data, _, diags := readAlertSlackChannelsHelper(t, server, overrides)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if reqCount.Load() != 1 {
		t.Errorf("expected exactly 1 HTTP request for manual pagination, got %d", reqCount.Load())
	}

	if data.PageToken.ValueString() != "next-page-token" {
		t.Errorf("expected PageToken 'next-page-token', got %v", data.PageToken)
	}

	if data.RowCount.ValueInt64() != 10 {
		t.Errorf("expected RowCount 10, got %d", data.RowCount.ValueInt64())
	}
}

func TestAlertSlackChannelsDataSource_Read_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := models.AlertSlackChannelList{
			HasSharedChannel:     false,
			ChannelsComplete:     true,
			IsWorkspaceConnected: false,
			RowCount:             new(0),
			WorkspaceStatus:      "none",
			PageToken:            nil,
			Items:                []models.AlertSlackChannel{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	data, _, diags := readAlertSlackChannelsHelper(t, server, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Items.IsNull() {
		t.Errorf("expected Items to be empty list, but got null")
	}

	var items []datasource_alert_slack_channels.ItemsValue
	diags = data.Items.ElementsAs(t.Context(), &items, false)
	if diags.HasError() {
		t.Fatalf("failed to decode items: %v", diags)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}

	if data.RowCount.ValueInt64() != 0 {
		t.Errorf("expected RowCount 0, got %d", data.RowCount.ValueInt64())
	}
}

func TestAlertSlackChannelsDataSource_Read_NameContainsForwarded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		overrides map[string]tftypes.Value
	}{
		{
			name: "auto",
			overrides: map[string]tftypes.Value{
				"name_contains": tftypes.NewValue(tftypes.String, "prod"),
			},
		},
		{
			name: "manual",
			overrides: map[string]tftypes.Value{
				"name_contains": tftypes.NewValue(tftypes.String, "prod"),
				"max_results":   tftypes.NewValue(tftypes.Number, 5),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var reqCount atomic.Int32
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqCount.Add(1)
				if got := r.URL.Query().Get("nameContains"); got != "prod" {
					t.Errorf("expected nameContains %q, got %q", "prod", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(models.AlertSlackChannelList{
					ChannelsComplete:     true,
					IsWorkspaceConnected: true,
					WorkspaceStatus:      "ok",
					Items:                []models.AlertSlackChannel{{Id: "C001", Name: new("prod-alerts")}},
				})
			}))

			_, _, diags := readAlertSlackChannelsHelper(t, server, tt.overrides)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if reqCount.Load() != 1 {
				t.Errorf("expected 1 HTTP request, got %d", reqCount.Load())
			}
		})
	}
}

// slackChannelsPartialServer serves two pages; the page at partialPage (1-based)
// reports channelsComplete=false.
func slackChannelsPartialServer(t *testing.T, partialPage int32) *httptest.Server {
	t.Helper()

	var reqCount atomic.Int32
	return httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		wantToken := ""
		if page > 1 {
			wantToken = "page-2"
		}
		if got := r.URL.Query().Get("pageToken"); got != wantToken {
			t.Errorf("request %d: expected pageToken %q, got %q", page, wantToken, got)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := models.AlertSlackChannelList{
			IsWorkspaceConnected: true,
			ChannelsComplete:     page != partialPage,
			RowCount:             new(2),
			WorkspaceStatus:      "ok",
			Items:                []models.AlertSlackChannel{{Id: fmt.Sprintf("C00%d", page)}},
		}
		if page == 1 {
			resp.PageToken = new("page-2")
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func requireIncompleteWarning(t *testing.T, diags diag.Diagnostics) {
	t.Helper()

	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if diags.WarningsCount() != 1 {
		t.Fatalf("expected exactly 1 warning, got %v", diags)
	}
	if got := diags.Warnings()[0].Summary(); got != "Incomplete Slack Channel List" {
		t.Errorf("unexpected warning summary %q", got)
	}
	for _, cause := range []string{"rate limit", "private-channel scope", "service-account"} {
		if got := diags.Warnings()[0].Detail(); !strings.Contains(got, cause) {
			t.Errorf("expected warning detail to mention %q, got %q", cause, got)
		}
	}
}

func TestAlertSlackChannelsDataSource_Read_IncompleteWarning_Auto(t *testing.T) {
	t.Parallel()

	// Only the last page is partial: the aggregate must still be incomplete.
	for _, partialPage := range []int32{1, 2} {
		t.Run(fmt.Sprintf("page_%d", partialPage), func(t *testing.T) {
			t.Parallel()

			server := slackChannelsPartialServer(t, partialPage)
			data, _, diags := readAlertSlackChannelsHelper(t, server, nil)

			requireIncompleteWarning(t, diags)
			if data.ChannelsComplete.IsNull() || data.ChannelsComplete.ValueBool() {
				t.Errorf("expected ChannelsComplete=false, got %v", data.ChannelsComplete)
			}
			// State is still populated despite the warning.
			if len(data.Items.Elements()) != 2 {
				t.Errorf("expected 2 items in state, got %d", len(data.Items.Elements()))
			}
		})
	}
}

func TestAlertSlackChannelsDataSource_Read_IncompleteWarning_Manual(t *testing.T) {
	t.Parallel()

	server := slackChannelsPartialServer(t, 1)
	overrides := map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, 1),
	}
	data, _, diags := readAlertSlackChannelsHelper(t, server, overrides)

	requireIncompleteWarning(t, diags)
	if data.ChannelsComplete.IsNull() || data.ChannelsComplete.ValueBool() {
		t.Errorf("expected ChannelsComplete=false, got %v", data.ChannelsComplete)
	}
}

// slackChannelsScriptedServer answers each request with the page registered for
// its incoming pageToken ("" is the first page) and counts the requests. An
// unregistered token fails the test.
func slackChannelsScriptedServer(t *testing.T, pages map[string]models.AlertSlackChannelList) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var reqCount atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		token := r.URL.Query().Get("pageToken")
		page, ok := pages[token]
		if !ok {
			t.Errorf("unexpected pageToken %q", token)
			http.Error(w, "unexpected token", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	}))
	return server, &reqCount
}

func slackChannelsPage(token *string, ids ...string) models.AlertSlackChannelList {
	items := make([]models.AlertSlackChannel, 0, len(ids))
	for _, id := range ids {
		items = append(items, models.AlertSlackChannel{Id: id, Name: new("name-" + id)})
	}
	return models.AlertSlackChannelList{
		IsWorkspaceConnected: true,
		ChannelsComplete:     true,
		WorkspaceStatus:      "ok",
		PageToken:            token,
		Items:                items,
	}
}

func slackChannelItemIDs(t *testing.T, data alertSlackChannelsDataSourceModel) []string {
	t.Helper()

	var items []datasource_alert_slack_channels.ItemsValue
	if diags := data.Items.ElementsAs(t.Context(), &items, false); diags.HasError() {
		t.Fatalf("failed to decode items: %v", diags)
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id.ValueString())
	}
	return ids
}

// The API omits rowCount unless a single request saw the whole list, so a page
// that has more pages must leave row_count null rather than report the page size.
func TestAlertSlackChannelsDataSource_Read_ManualPagination_RowCountOmitted(t *testing.T) {
	t.Parallel()

	server, reqCount := slackChannelsScriptedServer(t, map[string]models.AlertSlackChannelList{
		"": slackChannelsPage(new("next"), "C1", "C2"),
	})
	data, _, diags := readAlertSlackChannelsHelper(t, server, map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, 2),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if reqCount.Load() != 1 {
		t.Errorf("expected 1 request, got %d", reqCount.Load())
	}
	if !data.RowCount.IsNull() {
		t.Errorf("expected RowCount to be null when the API omits rowCount, got %v", data.RowCount)
	}
	if data.PageToken.ValueString() != "next" {
		t.Errorf("expected PageToken 'next', got %v", data.PageToken)
	}
}

// A filtered page can be empty while pageToken is still set; manual mode returns
// it as is, and Items is an empty list, never null.
func TestAlertSlackChannelsDataSource_Read_ManualPagination_EmptyPageWithToken(t *testing.T) {
	t.Parallel()

	server, _ := slackChannelsScriptedServer(t, map[string]models.AlertSlackChannelList{
		"start": slackChannelsPage(new("more")),
	})
	data, _, diags := readAlertSlackChannelsHelper(t, server, map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, 10),
		"page_token":  tftypes.NewValue(tftypes.String, "start"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Items.IsNull() || len(data.Items.Elements()) != 0 {
		t.Errorf("expected an empty, non-null Items list, got %v", data.Items)
	}
	if data.PageToken.ValueString() != "more" {
		t.Errorf("expected PageToken 'more', got %v", data.PageToken)
	}
	if !data.RowCount.IsNull() {
		t.Errorf("expected RowCount to be null, got %v", data.RowCount)
	}
}

// Auto mode keeps following pageToken through empty pages, derives row_count from
// what it fetched (no page carries rowCount) and keeps the API's order.
func TestAlertSlackChannelsDataSource_Read_AutoPagination_EmptyPagesFollowed(t *testing.T) {
	t.Parallel()

	server, reqCount := slackChannelsScriptedServer(t, map[string]models.AlertSlackChannelList{
		"":   slackChannelsPage(new("t2")),
		"t2": slackChannelsPage(new("t3")),
		"t3": slackChannelsPage(nil, "C9", "C1"),
	})
	data, _, diags := readAlertSlackChannelsHelper(t, server, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if reqCount.Load() != 3 {
		t.Errorf("expected 3 requests, got %d", reqCount.Load())
	}
	if data.RowCount.ValueInt64() != 2 {
		t.Errorf("expected RowCount 2, got %v", data.RowCount)
	}
	if !data.PageToken.IsNull() {
		t.Errorf("expected PageToken to be null in auto mode, got %v", data.PageToken)
	}
	// Deliberately unsorted: the API order is the contract, not name or ID order.
	if got, want := strings.Join(slackChannelItemIDs(t, data), ","), "C9,C1"; got != want {
		t.Errorf("expected items in API order %q, got %q", want, got)
	}
}

func TestAlertSlackChannelsDataSource_Read_InvalidPageToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		overrides map[string]tftypes.Value
		wantHint  bool
	}{
		{
			name: "manual with page_token",
			overrides: map[string]tftypes.Value{
				"max_results": tftypes.NewValue(tftypes.Number, 5),
				"page_token":  tftypes.NewValue(tftypes.String, "stale"),
			},
			wantHint: true,
		},
		{
			name: "auto from page_token",
			overrides: map[string]tftypes.Value{
				"page_token": tftypes.NewValue(tftypes.String, "stale"),
			},
			wantHint: true,
		},
		{
			name:      "no page_token",
			overrides: nil,
			wantHint:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error":"invalid pageToken"}`, http.StatusBadRequest)
			}))
			// The provider's retry client reports a 400 as an error, which is the
			// path a real read takes; a plain client hands the response back instead.
			retryClient, err := newClientWithHTTPClient(server.URL, "test-token", "", "1.9.0", "dev", server.Client())
			if err != nil {
				t.Fatalf("failed to create retry client: %v", err)
			}
			_, _, diags := readAlertSlackChannelsWithClient(t, retryClient, tt.overrides)
			if !diags.HasError() {
				t.Fatal("expected an error")
			}
			checkDetail(t, diags.Errors()[0].Detail(), tt.wantHint)

			_, _, diags = readAlertSlackChannelsHelper(t, server, tt.overrides)
			if !diags.HasError() {
				t.Fatal("expected an error")
			}
			checkDetail(t, diags.Errors()[0].Detail(), tt.wantHint)
		})
	}
}

func checkDetail(t *testing.T, detail string, wantHint bool) {
	t.Helper()

	const restartHint = "read again without page_token"
	if !strings.Contains(detail, "invalid pageToken") {
		t.Errorf("expected the API body in the error, got %q", detail)
	}
	if got := strings.Contains(detail, restartHint); got != wantHint {
		t.Errorf("restart hint present = %v, want %v (detail %q)", got, wantHint, detail)
	}
}
