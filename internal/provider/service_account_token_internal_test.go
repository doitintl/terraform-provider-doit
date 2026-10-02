package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

func TestMapServiceAccountTokenToModel(t *testing.T) {
	// Non-UTC offset in the API value must be normalized to UTC.
	loc := time.FixedZone("plus2", 2*60*60)
	expires := time.Date(2027, 6, 1, 10, 0, 0, 123456000, loc)
	created := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	resp := &models.ServiceAccountToken{
		Id:               valueToNullable("tok-1"),
		ServiceAccountId: new("sa-1"),
		CustomerId:       new("cust-1"),
		Name:             "ci",
		State:            models.ServiceAccountTokenState("disabled"),
		CreateTime:       valueToNullable(created),
		ExpiresTime:      valueToNullable(expires),
	}

	// Prior access_token must survive a read.
	state := serviceAccountTokenResourceModel{
		AccessToken: types.StringValue("secret")}
	mapServiceAccountTokenToModel(resp, &state)

	checks := map[string][2]string{
		"id":                 {state.Id.ValueString(), "tok-1"},
		"service_account_id": {state.ServiceAccountId.ValueString(), "sa-1"},
		"customer_id":        {state.CustomerId.ValueString(), "cust-1"},
		"name":               {state.Name.ValueString(), "ci"},
		"state":              {state.State.ValueString(), "disabled"},
		"create_time":        {state.CreateTime.ValueString(), "2026-09-01T08:00:00Z"},
		"expires_time":       {state.ExpiresTime.ValueString(), "2027-06-01T08:00:00Z"},
		"access_token":       {state.AccessToken.ValueString(), "secret"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if !state.LastUsedTime.IsNull() {
		t.Errorf("last_used_time = %v, want null", state.LastUsedTime)
	}
}

func TestMapServiceAccountTokenToModel_NullExpiryKeepsServiceAccountID(t *testing.T) {
	var nullExpiry nullable.Nullable[time.Time]
	nullExpiry.SetNull()
	resp := &models.ServiceAccountToken{
		Id:          valueToNullable("tok-1"),
		Name:        "ci",
		State:       models.ServiceAccountTokenState("active"),
		ExpiresTime: nullExpiry,
	}

	state := serviceAccountTokenResourceModel{
		ServiceAccountId: types.StringValue("sa-from-import")}
	mapServiceAccountTokenToModel(resp, &state)

	if !state.ExpiresTime.IsNull() {
		t.Errorf("expires_time = %v, want null", state.ExpiresTime)
	}
	if state.ServiceAccountId.ValueString() != "sa-from-import" {
		t.Errorf("service_account_id = %q, want it preserved when the response omits it", state.ServiceAccountId.ValueString())
	}
}

func TestOverlayServiceAccountTokenComputedFields(t *testing.T) {
	expires := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	lastUsed := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	resp := &models.ServiceAccountToken{
		Id:               valueToNullable("tok-1"),
		ServiceAccountId: new("sa-1"),
		CustomerId:       new("cust-1"),
		Name:             "ci",
		State:            models.ServiceAccountTokenState("active"),
		CreateTime:       valueToNullable(time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)),
		ExpiresTime:      valueToNullable(expires),
		LastUsedTime:     valueToNullable(lastUsed),
	}

	t.Run("create resolves unknowns", func(t *testing.T) {
		plan := serviceAccountTokenResourceModel{
			ServiceAccountId: types.StringValue("sa-1"),
			Name:             types.StringValue("ci"),
			Id:               types.StringUnknown(),
			CustomerId:       types.StringUnknown(),
			CreateTime:       types.StringUnknown(),
			ExpiresTime:      types.StringUnknown(),
			State:            types.StringUnknown(),
			LastUsedTime:     types.StringUnknown(),
			AccessToken:      types.StringValue("one-time-secret")}

		overlayServiceAccountTokenComputedFields(resp, &plan)

		if plan.Id.ValueString() != "tok-1" || plan.CustomerId.ValueString() != "cust-1" {
			t.Errorf("computed IDs not resolved: id=%v customer_id=%v", plan.Id, plan.CustomerId)
		}
		if plan.ExpiresTime.ValueString() != "2027-01-01T00:00:00Z" || plan.State.ValueString() != "active" {
			t.Errorf("optional+computed not resolved: expires_time=%v state=%v", plan.ExpiresTime, plan.State)
		}
		if plan.LastUsedTime.ValueString() != "2026-10-02T09:00:00Z" {
			t.Errorf("last_used_time = %v", plan.LastUsedTime)
		}
		if plan.AccessToken.ValueString() != "one-time-secret" {
			t.Errorf("access_token = %v, want preserved", plan.AccessToken)
		}
	})

	t.Run("update preserves known plan values", func(t *testing.T) {
		plan := serviceAccountTokenResourceModel{
			ServiceAccountId: types.StringValue("sa-1"),
			Name:             types.StringValue("ci"),
			Id:               types.StringValue("tok-1"),
			CustomerId:       types.StringValue("cust-1"),
			CreateTime:       types.StringValue("2026-09-01T08:00:00Z"),
			ExpiresTime:      types.StringValue("2027-01-01T00:00:00Z"),
			State:            types.StringValue("disabled"),
			LastUsedTime:     types.StringValue("2026-09-15T00:00:00Z"),
			AccessToken:      types.StringValue("one-time-secret")}

		overlayServiceAccountTokenComputedFields(resp, &plan)

		if plan.State.ValueString() != "disabled" {
			t.Errorf("state = %v, want the planned value kept", plan.State)
		}
		if plan.LastUsedTime.ValueString() != "2026-09-15T00:00:00Z" {
			t.Errorf("known last_used_time = %v, want it kept", plan.LastUsedTime)
		}
		if plan.AccessToken.ValueString() != "one-time-secret" {
			t.Errorf("access_token = %v, want preserved", plan.AccessToken)
		}
	})

	t.Run("unknown access token never stays unknown", func(t *testing.T) {
		plan := serviceAccountTokenResourceModel{
			AccessToken: types.StringUnknown()}
		overlayServiceAccountTokenComputedFields(resp, &plan)
		if plan.AccessToken.IsUnknown() {
			t.Error("access_token must not remain unknown after apply")
		}
	})
}

func TestServiceAccountTokenToCreateRequest(t *testing.T) {
	t.Run("omitted expires_time", func(t *testing.T) {
		plan := serviceAccountTokenResourceModel{
			Name:        types.StringValue("ci"),
			ExpiresTime: types.StringUnknown()}

		req, diags := plan.toCreateRequest()
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if req.Name != "ci" || req.ExpiresTime != nil {
			t.Errorf("req = %+v, want name only", req)
		}
	})

	t.Run("explicit expires_time", func(t *testing.T) {
		plan := serviceAccountTokenResourceModel{
			Name:        types.StringValue("ci"),
			ExpiresTime: types.StringValue("2027-01-01T00:00:00Z")}

		req, diags := plan.toCreateRequest()
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if req.ExpiresTime == nil || !req.ExpiresTime.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expires_time = %v", req.ExpiresTime)
		}
	})
}

func TestServiceAccountTokenToUpdateRequest(t *testing.T) {
	plan := serviceAccountTokenResourceModel{
		State: types.StringValue("disabled")}
	if got := plan.toUpdateRequest().State; got != "disabled" {
		t.Errorf("state = %q, want disabled", got)
	}
}

func TestServiceAccountTokenExpiresTimePattern(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"2027-01-01T00:00:00Z", true},
		{"2027-01-01T00:00:00+00:00", false},
		{"2027-06-01T10:00:00+02:00", false},
		{"2027-01-01T00:00:00.123Z", false},
		{"2027-01-01", false},
		{"2027-01-01 00:00:00Z", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := serviceAccountTokenExpiresTimePattern.MatchString(tt.value); got != tt.valid {
			t.Errorf("pattern match for %q = %v, want %v", tt.value, got, tt.valid)
		}
	}
}

func TestServiceAccountTokenImportState_InvalidID(t *testing.T) {
	r := &serviceAccountTokenResource{}
	for _, id := range []string{"tok-only", "/tok", "sa/", "a/b/c", ""} {
		resp := &resource.ImportStateResponse{}
		r.ImportState(t.Context(), resource.ImportStateRequest{ID: id}, resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("import ID %q: expected an error", id)
		}
	}
}

func TestServiceAccountToken_PopulateState_NotFound(t *testing.T) {
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/iam/v1/service-accounts/sa-1/api-tokens/missing" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	res := &serviceAccountTokenResource{client: newTestAPIClient(t, srv)}
	state := serviceAccountTokenResourceModel{
		ServiceAccountId: types.StringValue("sa-1"),
		Id:               types.StringValue("missing")}

	if diags := res.populateState(t.Context(), &state); diags.HasError() {
		t.Fatalf("populateState should not return error on 404, got: %v", diags)
	}
	if !state.Id.IsNull() {
		t.Errorf("expected state.Id to be null on 404, got %q", state.Id.ValueString())
	}
}

// The server-assigned default expiry carries sub-second precision (nanoseconds
// on create, microseconds on GET). Both must format to the same whole second so
// state never flips between the create and read values.
func TestMapServiceAccountTokenToModel_DefaultExpiryFractionalSeconds(t *testing.T) {
	create := time.Date(2027, 10, 1, 6, 47, 25, 248788386, time.UTC)
	read := create.Truncate(time.Microsecond)

	var fromCreate, fromRead serviceAccountTokenResourceModel
	mapServiceAccountTokenToModel(&models.ServiceAccountToken{ExpiresTime: valueToNullable(create)}, &fromCreate)
	mapServiceAccountTokenToModel(&models.ServiceAccountToken{ExpiresTime: valueToNullable(read)}, &fromRead)

	if fromCreate.ExpiresTime.ValueString() != "2027-10-01T06:47:25Z" || fromRead.ExpiresTime != fromCreate.ExpiresTime {
		t.Errorf("expires_time create=%v read=%v, want both 2027-10-01T06:47:25Z", fromCreate.ExpiresTime, fromRead.ExpiresTime)
	}
}

// expires_time combines the canonical-format regex with the RFC 3339 validator:
// the regex alone accepts impossible values like month 99.
func TestServiceAccountTokenExpiresTimeValidators(t *testing.T) {
	ctx := t.Context()

	r := &serviceAccountTokenResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	attr, ok := schemaResp.Schema.Attributes["expires_time"].(schema.StringAttribute)
	if !ok {
		t.Fatal("expires_time is not a string attribute")
	}

	tests := []struct {
		name  string
		value types.String
		want  int // number of error diagnostics
	}{
		{"canonical", types.StringValue("2027-01-01T00:00:00Z"), 0},
		{"null", types.StringNull(), 0},
		{"unknown", types.StringUnknown(), 0},
		{"offset", types.StringValue("2027-06-01T10:00:00+02:00"), 1},
		{"fractional seconds", types.StringValue("2027-06-01T08:00:00.123Z"), 1},
		{"impossible month and day", types.StringValue("2027-99-99T00:00:00Z"), 1},
		{"impossible time", types.StringValue("2027-01-01T99:99:99Z"), 1},
		{"not a timestamp", types.StringValue("tomorrow"), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errs int
			for _, v := range attr.Validators {
				resp := &validator.StringResponse{}
				v.ValidateString(ctx, validator.StringRequest{Path: path.Root("expires_time"), ConfigValue: tt.value}, resp)
				errs += resp.Diagnostics.ErrorsCount()
			}
			if errs != tt.want {
				t.Errorf("error count = %d, want %d", errs, tt.want)
			}
		})
	}
}
