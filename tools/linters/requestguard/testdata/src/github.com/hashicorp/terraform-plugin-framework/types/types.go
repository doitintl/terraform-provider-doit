// Stub types package for analysistest.
package types

type String struct{}
type Float64 struct{}
type Bool struct{}
type List struct{}
type Int64 struct{}

func (s String) IsUnknown() bool  { return false }
func (s String) IsNull() bool     { return false }
func (f Float64) IsUnknown() bool { return false }
func (f Float64) IsNull() bool    { return false }
func (b Bool) IsUnknown() bool    { return false }
func (b Bool) IsNull() bool       { return false }
func (l List) IsUnknown() bool    { return false }
func (l List) IsNull() bool       { return false }
func (i Int64) IsUnknown() bool   { return false }
func (i Int64) IsNull() bool      { return false }

func (s String) ValueString() string            { return "" }
func (s String) ValueStringPointer() *string    { return nil }
func (f Float64) ValueFloat64() float64         { return 0 }
func (f Float64) ValueFloat64Pointer() *float64 { return nil }
func (b Bool) ValueBool() bool                  { return false }
func (b Bool) ValueBoolPointer() *bool          { return nil }
func (i Int64) ValueInt64() int64               { return 0 }
func (i Int64) ValueInt64Pointer() *int64       { return nil }

var StringType interface{} = nil

func Int64Value(v int64) Int64 { return Int64{} }
func Int64Null() Int64         { return Int64{} }
