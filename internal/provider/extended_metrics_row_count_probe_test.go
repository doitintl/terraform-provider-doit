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

	got, err := probeExtendedMetrics(t.Context(), client)
	if err != nil {
		t.Fatal(err)
	}
	if got.count < 2 {
		t.Errorf("probe = %d for a tenant with %d metrics and no rowCount; the pagination tests (need at least 2) would skip", got.count, total)
	}
	if got.apiTotal {
		t.Error("the probe reports an API total although the API sent no rowCount")
	}
}

func TestProbeExtendedMetricsRowCount_WithRowCount(t *testing.T) {
	const total = 5
	client := newExtendedMetricsProbeClient(t, total, true)

	got, err := probeExtendedMetrics(t.Context(), client)
	if err != nil {
		t.Fatal(err)
	}
	if got.count != total || !got.apiTotal {
		t.Errorf("probe = %+v, want the API's row count %d as an API total", got, total)
	}
}

func TestCheckExtendedMetricsPaginationRowCounts(t *testing.T) {
	for _, tc := range []struct {
		name                                                 string
		firstRowCount, firstItems, tokenRowCount, tokenItems string
		apiTotal                                             bool
		wantErr                                              bool
	}{
		{"API total, same for both reads", "3", "1", "3", "2", true, false},
		{"API total, reads disagree", "3", "1", "2", "2", true, true},
		{"no API total, each read reports its items", "1", "1", "2", "2", false, false},
		{"no API total, the read from the token misreports", "1", "1", "1", "2", false, true},
		{"no API total, the first page misreports", "3", "1", "2", "2", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkExtendedMetricsPaginationRowCounts(tc.firstRowCount, tc.firstItems, tc.tokenRowCount, tc.tokenItems, tc.apiTotal)
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
