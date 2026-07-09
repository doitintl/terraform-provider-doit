package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

	longTag := ""
	for range 81 {
		longTag += "a"
	}
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
			state.Tags.ElementsAs(ctx, &gotTags, false)
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
