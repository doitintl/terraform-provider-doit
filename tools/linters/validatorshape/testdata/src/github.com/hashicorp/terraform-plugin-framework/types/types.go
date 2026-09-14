package types

import "github.com/hashicorp/terraform-plugin-framework/attr"

type String struct{}

var _ attr.Value = String{}

func StringUnknown() String        { return String{} }
func (String) IsUnknown() bool     { return false }
func (String) IsNull() bool        { return false }
func (String) ValueString() string { return "" }
