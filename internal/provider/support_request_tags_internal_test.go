package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestReconcileTags verifies the Read-path reconciliation that preserves the
// user's tag representation when the API returns a normalized (trim + lowercase)
// form of the same tag, while still surfacing genuine external changes.
func TestReconcileTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		apiTags   []string
		priorTags []string
		want      []string
	}{
		{
			name:      "normalized value preserves user spelling",
			apiTags:   []string{"billing", "review"},
			priorTags: []string{"Billing", "review"},
			want:      []string{"Billing", "review"},
		},
		{
			name:      "trimmed value preserves user spelling",
			apiTags:   []string{"urgent"},
			priorTags: []string{"  Urgent  "},
			want:      []string{"  Urgent  "},
		},
		{
			name:      "no prior state takes API values as-is (import)",
			apiTags:   []string{"billing", "review"},
			priorTags: nil,
			want:      []string{"billing", "review"},
		},
		{
			name:      "externally added tag is taken as-is",
			apiTags:   []string{"billing", "newtag"},
			priorTags: []string{"Billing"},
			want:      []string{"Billing", "newtag"},
		},
		{
			name:      "externally removed tag is dropped (drift detected)",
			apiTags:   []string{"billing"},
			priorTags: []string{"Billing", "Review"},
			want:      []string{"Billing"},
		},
		{
			name:      "empty API response yields empty set",
			apiTags:   []string{},
			priorTags: []string{"Billing"},
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := reconcileTags(tt.apiTags, tt.priorTags)
			if len(got) != len(tt.want) {
				t.Fatalf("reconcileTags() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("reconcileTags()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestSupportRequestTagsTagValidators verifies the tags attribute enforces the
// TagsRequest constraints (1-50 tags, each 1-80 chars, not whitespace-only) at
// plan time, mirroring the OpenAPI spec.
func TestSupportRequestTagsTagValidators(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := &supportRequestTagsResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors: %v", schemaResp.Diagnostics)
	}

	tagsAttr, ok := schemaResp.Schema.Attributes["tags"].(schema.SetAttribute)
	if !ok {
		t.Fatalf("tags attribute is not a SetAttribute: %T", schemaResp.Schema.Attributes["tags"])
	}
	validators := tagsAttr.SetValidators()
	if len(validators) == 0 {
		t.Fatal("expected tags attribute to declare validators")
	}

	longTag := strings.Repeat("a", 81)
	manyTags := make([]string, 51)
	for i := range manyTags {
		manyTags[i] = fmt.Sprintf("tag-%d", i)
	}

	tests := []struct {
		name        string
		tags        []string
		expectError bool
	}{
		{name: "valid pair", tags: []string{"billing", "review"}, expectError: false},
		{name: "single tag", tags: []string{"billing"}, expectError: false},
		{name: "empty set allowed (clears tags)", tags: []string{}, expectError: false},
		{name: "over 50 tags rejected", tags: manyTags, expectError: true},
		{name: "tag over 80 chars rejected", tags: []string{longTag}, expectError: true},
		{name: "whitespace-only tag rejected", tags: []string{"   "}, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			set, d := types.SetValueFrom(ctx, types.StringType, tt.tags)
			if d.HasError() {
				t.Fatalf("failed to build set: %v", d)
			}

			var diags diag.Diagnostics
			for _, v := range validators {
				resp := &validator.SetResponse{}
				v.ValidateSet(ctx, validator.SetRequest{
					Path:        path.Root("tags"),
					ConfigValue: set,
				}, resp)
				diags.Append(resp.Diagnostics...)
			}

			if diags.HasError() != tt.expectError {
				t.Errorf("hasError = %v, expectError = %v; diagnostics: %v",
					diags.HasError(), tt.expectError, diags)
			}
		})
	}
}

// TestSupportRequestTagsCreate_Reconciles verifies that Create is authoritative:
// it reads the ticket's current visible tags and reconciles them to the desired
// set — removing tags that aren't desired (including clearing all when the
// desired set is empty) and adding those that are missing.
func TestSupportRequestTagsCreate_Reconciles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		currentTags []string
		desiredTags []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name:        "empty desired clears existing tags",
			currentTags: []string{"urgent", "billing"},
			desiredTags: []string{},
			wantAdded:   nil,
			wantRemoved: []string{"billing", "urgent"},
		},
		{
			name:        "reconciles to desired set",
			currentTags: []string{"billing", "old"},
			desiredTags: []string{"billing", "new"},
			wantAdded:   []string{"new"},
			wantRemoved: []string{"old"},
		},
		{
			name:        "empty desired on ticket with no tags is a no-op",
			currentTags: []string{},
			desiredTags: []string{},
			wantAdded:   nil,
			wantRemoved: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var mu sync.Mutex
			var added, removed []string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					_ = json.NewEncoder(w).Encode(map[string][]string{"tags": tt.currentTags})
				case http.MethodPost, http.MethodDelete:
					var body struct {
						Tags []string `json:"tags"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					mu.Lock()
					if r.Method == http.MethodPost {
						added = append(added, body.Tags...)
					} else {
						removed = append(removed, body.Tags...)
					}
					mu.Unlock()
					_ = json.NewEncoder(w).Encode(map[string][]string{"applied_tags": body.Tags})
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()

			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			r := &supportRequestTagsResource{client: client}
			ctx := context.Background()

			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

			createReq := resource.CreateRequest{
				Plan: buildTagsPlan(ctx, t, schemaResp.Schema, 123, tt.desiredTags),
			}
			createResp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Create(ctx, createReq, createResp)

			if createResp.Diagnostics.HasError() {
				t.Fatalf("Create returned errors: %v", createResp.Diagnostics)
			}

			sort.Strings(added)
			sort.Strings(removed)
			if !equalStrings(added, tt.wantAdded) {
				t.Errorf("added = %v, want %v", added, tt.wantAdded)
			}
			if !equalStrings(removed, tt.wantRemoved) {
				t.Errorf("removed = %v, want %v", removed, tt.wantRemoved)
			}
		})
	}
}

// buildTagsPlan constructs a Create/Update plan for the tags resource with the
// given ticket ID and desired tag set (id unknown, timeouts null).
func buildTagsPlan(ctx context.Context, t *testing.T, s schema.Schema, ticketID int64, tags []string) tfsdk.Plan {
	t.Helper()

	tagVals := make([]tftypes.Value, 0, len(tags))
	for _, tag := range tags {
		tagVals = append(tagVals, tftypes.NewValue(tftypes.String, tag))
	}

	planValues := map[string]tftypes.Value{
		"id":        tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"ticket_id": tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(ticketID)),
		"tags":      tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, tagVals),
		"timeouts":  tftypes.NewValue(s.Attributes["timeouts"].GetType().TerraformType(ctx), nil),
	}

	raw := tftypes.NewValue(
		tftypes.Object{AttributeTypes: getAttributeTypes(ctx, s.Attributes)},
		planValues,
	)
	return tfsdk.Plan{Schema: s, Raw: raw}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSupportRequestTagsPopulateState_Normalization verifies that populateState
// preserves the user's tag representation when the mocked GET /tags endpoint
// returns the trim+lowercased form, so no false drift is produced. It also
// covers the 404 (resource removed) path.
func TestSupportRequestTagsPopulateState_Normalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		priorTags    []string
		wantIDNull   bool
		wantTags     []string
	}{
		{
			name:         "API normalization preserved against prior state",
			statusCode:   http.StatusOK,
			responseBody: `{"tags":["billing","review"]}`,
			priorTags:    []string{"Billing", "review"},
			wantTags:     []string{"Billing", "review"},
		},
		{
			name:         "import with no prior state takes API values",
			statusCode:   http.StatusOK,
			responseBody: `{"tags":["billing"]}`,
			priorTags:    nil,
			wantTags:     []string{"billing"},
		},
		{
			name:         "404 removes resource from state",
			statusCode:   http.StatusNotFound,
			responseBody: `{"error":"Ticket not found"}`,
			priorTags:    []string{"Billing"},
			wantIDNull:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			r := &supportRequestTagsResource{client: client}

			ctx := context.Background()

			priorTagsSet := types.SetNull(types.StringType)
			if tt.priorTags != nil {
				var setDiags diag.Diagnostics
				priorTagsSet, setDiags = types.SetValueFrom(ctx, types.StringType, tt.priorTags)
				if setDiags.HasError() {
					t.Fatalf("Failed to build prior tags set: %v", setDiags)
				}
			}

			state := supportRequestTagsResourceModel{
				Id:       types.StringValue("123"),
				TicketId: types.Int64Value(123),
				Tags:     priorTagsSet,
			}

			diags := r.populateState(ctx, &state)
			if diags.HasError() {
				t.Fatalf("populateState returned errors: %v", diags)
			}

			if tt.wantIDNull {
				if !state.Id.IsNull() {
					t.Errorf("expected Id to be null (resource removed), got %q", state.Id.ValueString())
				}
				return
			}

			var gotTags []string
			if d := state.Tags.ElementsAs(ctx, &gotTags, false); d.HasError() {
				t.Fatalf("failed to read tags from state: %v", d)
			}
			if len(gotTags) != len(tt.wantTags) {
				t.Fatalf("populateState tags = %v, want %v", gotTags, tt.wantTags)
			}
			want := make(map[string]bool, len(tt.wantTags))
			for _, w := range tt.wantTags {
				want[w] = true
			}
			for _, g := range gotTags {
				if !want[g] {
					t.Errorf("unexpected tag %q in state; want set %v", g, tt.wantTags)
				}
			}
		})
	}
}
