package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestPointerAccessorsOnUnknown proves that Terraform Plugin Framework pointer
// accessors return a pointer to the zero value for Unknown, NOT nil.
// This is the core justification for why the requestguard linter flags
// pointer accessors: omitting an IsUnknown() guard sends a zero value
// to the API instead of omitting the field (via omitempty + nil).
func TestPointerAccessorsOnUnknown(t *testing.T) {
	t.Run("ValueBoolPointer on Unknown returns *false, not nil", func(t *testing.T) {
		unknown := types.BoolUnknown()
		got := unknown.ValueBoolPointer()
		if got == nil {
			t.Fatal("ValueBoolPointer() on Unknown returned nil; expected *false")
		}
		if *got != false {
			t.Fatalf("ValueBoolPointer() on Unknown returned *%v; expected *false", *got)
		}
	})

	t.Run("ValueBoolPointer on Null returns nil", func(t *testing.T) {
		null := types.BoolNull()
		got := null.ValueBoolPointer()
		if got != nil {
			t.Fatalf("ValueBoolPointer() on Null returned *%v; expected nil", *got)
		}
	})

	t.Run("ValueInt64Pointer on Unknown returns *0, not nil", func(t *testing.T) {
		unknown := types.Int64Unknown()
		got := unknown.ValueInt64Pointer()
		if got == nil {
			t.Fatal("ValueInt64Pointer() on Unknown returned nil; expected *0")
		}
		if *got != 0 {
			t.Fatalf("ValueInt64Pointer() on Unknown returned *%d; expected *0", *got)
		}
	})

	t.Run("ValueInt64Pointer on Null returns nil", func(t *testing.T) {
		null := types.Int64Null()
		got := null.ValueInt64Pointer()
		if got != nil {
			t.Fatalf("ValueInt64Pointer() on Null returned *%d; expected nil", *got)
		}
	})

	t.Run("ValueStringPointer on Unknown returns *empty, not nil", func(t *testing.T) {
		unknown := types.StringUnknown()
		got := unknown.ValueStringPointer()
		if got == nil {
			t.Fatal("ValueStringPointer() on Unknown returned nil; expected *\"\"")
		}
		if *got != "" {
			t.Fatalf("ValueStringPointer() on Unknown returned *%q; expected *\"\"", *got)
		}
	})

	t.Run("ValueStringPointer on Null returns nil", func(t *testing.T) {
		null := types.StringNull()
		got := null.ValueStringPointer()
		if got != nil {
			t.Fatalf("ValueStringPointer() on Null returned *%q; expected nil", *got)
		}
	})

	t.Run("ValueFloat64Pointer on Unknown returns *0.0, not nil", func(t *testing.T) {
		unknown := types.Float64Unknown()
		got := unknown.ValueFloat64Pointer()
		if got == nil {
			t.Fatal("ValueFloat64Pointer() on Unknown returned nil; expected *0.0")
		}
		if *got != 0.0 {
			t.Fatalf("ValueFloat64Pointer() on Unknown returned *%f; expected *0.0", *got)
		}
	})

	t.Run("ValueFloat64Pointer on Null returns nil", func(t *testing.T) {
		null := types.Float64Null()
		got := null.ValueFloat64Pointer()
		if got != nil {
			t.Fatalf("ValueFloat64Pointer() on Null returned *%f; expected nil", *got)
		}
	})
}

// TestPointerAccessorSerialization demonstrates the practical impact: when a
// struct field uses omitempty, a nil pointer is omitted from the JSON payload
// but a pointer to zero-value is serialized, sending unintended data to the API.
func TestPointerAccessorSerialization(t *testing.T) {
	type apiRequest struct {
		IncludeCurrent *bool   `json:"include_current,omitempty"`
		Amount         *int64  `json:"amount,omitempty"`
		Name           *string `json:"name,omitempty"`
	}

	t.Run("Null produces omitted fields (correct)", func(t *testing.T) {
		req := apiRequest{
			IncludeCurrent: types.BoolNull().ValueBoolPointer(),     // nil
			Amount:         types.Int64Null().ValueInt64Pointer(),   // nil
			Name:           types.StringNull().ValueStringPointer(), // nil
		}
		b, _ := json.Marshal(req)
		got := string(b)
		expected := "{}"
		if got != expected {
			t.Fatalf("Null fields should be omitted.\ngot:  %s\nwant: %s", got, expected)
		}
	})

	t.Run("Unknown produces zero-value fields (the bug)", func(t *testing.T) {
		req := apiRequest{
			IncludeCurrent: types.BoolUnknown().ValueBoolPointer(),     // *false
			Amount:         types.Int64Unknown().ValueInt64Pointer(),   // *0
			Name:           types.StringUnknown().ValueStringPointer(), // *""
		}
		b, _ := json.Marshal(req)
		got := string(b)
		// omitempty on pointer fields only omits nil, not pointers to zero
		// values. So all three zero-value pointers (*false, *0, *"") are
		// serialized — sending unintended data to the API.
		expected := `{"include_current":false,"amount":0,"name":""}`
		if got != expected {
			t.Fatalf("Unknown fields should NOT be serialized but they are.\ngot:  %s\nwant: %s", got, expected)
		}
	})

	t.Run("Guarded Unknown produces omitted fields (the fix)", func(t *testing.T) {
		req := apiRequest{}
		// With guards: only set fields that are Known
		boolVal := types.BoolUnknown()
		if !boolVal.IsNull() && !boolVal.IsUnknown() {
			req.IncludeCurrent = boolVal.ValueBoolPointer()
		}
		intVal := types.Int64Unknown()
		if !intVal.IsNull() && !intVal.IsUnknown() {
			req.Amount = intVal.ValueInt64Pointer()
		}
		strVal := types.StringUnknown()
		if !strVal.IsNull() && !strVal.IsUnknown() {
			req.Name = strVal.ValueStringPointer()
		}
		b, _ := json.Marshal(req)
		got := string(b)
		expected := "{}"
		if got != expected {
			t.Fatalf("Guarded Unknown fields should be omitted.\ngot:  %s\nwant: %s", got, expected)
		}
	})
}
