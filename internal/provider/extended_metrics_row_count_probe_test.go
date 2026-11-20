package provider_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// newExtendedMetricsProbeClient serves total extended metrics, honoring
// maxResults, with or without the optional rowCount.
func newExtendedMetricsProbeClient(t *testing.T, total int, withRowCount bool) *models.ClientWithResponses {
	t.Helper()
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := total
		if v := r.URL.Query().Get("maxResults"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			limit = min(n, total)
		}
		items := make([]string, 0, limit)
		for i := range limit {
			items = append(items, fmt.Sprintf(`{"key":"k%d","label":"L%d","dataSource":"billing"}`, i, i))
		}
		body := `{"items":[` + strings.Join(items, ",") + `]`
		if limit < total {
			body += `,"pageToken":"next"`
		}
		if withRowCount {
			body += fmt.Sprintf(`,"rowCount":%d`, total)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body + "}"))
	}))
	// Start the server before reading server.URL.
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// The row count may be omitted (the list envelope makes it best-effort), so
// the probe the pagination tests use to decide whether to skip must still
// report that the tenant has several metrics.
func TestProbeExtendedMetricsRowCount_WithoutRowCount(t *testing.T) {
	const total = 5
	client := newExtendedMetricsProbeClient(t, total, false)

	got, err := probeExtendedMetricsRowCount(t.Context(), client)
	if err != nil {
		t.Fatal(err)
	}
	if got < 2 {
		t.Errorf("probe = %d for a tenant with %d metrics and no rowCount; the pagination tests (need at least 2) would skip", got, total)
	}
}

func TestProbeExtendedMetricsRowCount_WithRowCount(t *testing.T) {
	const total = 5
	client := newExtendedMetricsProbeClient(t, total, true)

	got, err := probeExtendedMetricsRowCount(t.Context(), client)
	if err != nil {
		t.Fatal(err)
	}
	if got != total {
		t.Errorf("probe = %d, want the API's row count %d", got, total)
	}
}
