package provider

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// allocationsQueryRecorder serves two pages of allocations and records the
// query of every request.
type allocationsQueryRecorder struct {
	mu      sync.Mutex
	queries []url.Values
}

func (s *allocationsQueryRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.queries = append(s.queries, r.URL.Query())
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"allocations": [{"id": "a1", "name": "one"}], "pageToken": "t2", "rowCount": 1}`))
			return
		}
		_, _ = w.Write([]byte(`{"allocations": [{"id": "a2", "name": "two"}], "rowCount": 1}`))
	})
}

func (s *allocationsQueryRecorder) requests() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.queries...)
}

func readAllocationsQueries(t *testing.T, overrides map[string]tftypes.Value) []url.Values {
	t.Helper()

	recorder := &allocationsQueryRecorder{}
	server := httptest.NewTestServer(t, recorder.handler())
	// NewTestServer starts on the first Client call, which assigns server.URL.
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := &allocationsDataSource{client: client}
	ctx := t.Context()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}

	configValues := map[string]tftypes.Value{}
	attrTypes := map[string]tftypes.Type{}
	for name, a := range schemaResp.Schema.Attributes {
		tfType := a.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if override, ok := overrides[name]; ok {
			configValues[name] = override
			continue
		}
		configValues[name] = tftypes.NewValue(tfType, nil)
	}

	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, configValues),
	}
	readResp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: config}, readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read() returned diagnostics: %v", readResp.Diagnostics)
	}
	return recorder.requests()
}

// TestAllocationsDataSource_SortParameters checks that a configured sort_by and
// sort_order reach every page request in both pagination modes.
func TestAllocationsDataSource_SortParameters(t *testing.T) {
	t.Parallel()

	sort := map[string]tftypes.Value{
		"sort_by":    tftypes.NewValue(tftypes.String, "name"),
		"sort_order": tftypes.NewValue(tftypes.String, "desc"),
	}

	t.Run("auto pagination", func(t *testing.T) {
		t.Parallel()
		reqs := readAllocationsQueries(t, sort)
		if len(reqs) != 2 {
			t.Fatalf("requests = %d, want 2", len(reqs))
		}
		for i, q := range reqs {
			if got := q.Get("sortBy"); got != "name" {
				t.Errorf("request %d: sortBy = %q, want %q", i, got, "name")
			}
			if got := q.Get("sortOrder"); got != "desc" {
				t.Errorf("request %d: sortOrder = %q, want %q", i, got, "desc")
			}
		}
	})

	t.Run("manual pagination", func(t *testing.T) {
		t.Parallel()
		overrides := map[string]tftypes.Value{
			"max_results": tftypes.NewValue(tftypes.Number, 1),
		}
		maps.Copy(overrides, sort)
		reqs := readAllocationsQueries(t, overrides)
		if len(reqs) != 1 {
			t.Fatalf("requests = %d, want 1", len(reqs))
		}
		if got := reqs[0].Get("sortBy"); got != "name" {
			t.Errorf("sortBy = %q, want %q", got, "name")
		}
		if got := reqs[0].Get("sortOrder"); got != "desc" {
			t.Errorf("sortOrder = %q, want %q", got, "desc")
		}
	})

	t.Run("unset", func(t *testing.T) {
		t.Parallel()
		reqs := readAllocationsQueries(t, nil)
		if len(reqs) != 2 {
			t.Fatalf("requests = %d, want 2", len(reqs))
		}
		for i, q := range reqs {
			if q.Has("sortBy") || q.Has("sortOrder") {
				t.Errorf("request %d: query %q carries sort parameters although none are configured", i, q.Encode())
			}
		}
	})
}
