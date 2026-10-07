package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			token := "token-page-2"
			resp := models.AlertSlackChannelList{
				HasSharedChannel:     true,
				ChannelsComplete:     true,
				IsWorkspaceConnected: true,
				RowCount:             3,
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
			RowCount:             3,
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
			RowCount:             10,
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
			RowCount:             0,
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

// slackChannelsPartialServer serves two pages; the page at partialPage (1-based)
// reports channelsComplete=false.
func slackChannelsPartialServer(t *testing.T, partialPage int32) *httptest.Server {
	t.Helper()

	var reqCount atomic.Int32
	return httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		resp := models.AlertSlackChannelList{
			IsWorkspaceConnected: true,
			ChannelsComplete:     page != partialPage,
			RowCount:             2,
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
