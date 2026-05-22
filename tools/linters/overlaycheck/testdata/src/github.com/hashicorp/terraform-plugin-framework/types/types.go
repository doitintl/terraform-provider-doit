// Stub types package for analysistest.
package types

type String struct{}
type Float64 struct{}
type Bool struct{}
type List struct{}

func (s String) IsUnknown() bool  { return false }
func (s String) IsNull() bool     { return false }
func (f Float64) IsUnknown() bool { return false }
func (f Float64) IsNull() bool    { return false }
func (b Bool) IsUnknown() bool    { return false }
func (b Bool) IsNull() bool       { return false }
func (l List) IsUnknown() bool    { return false }
func (l List) IsNull() bool       { return false }

var StringType interface{} = nil
