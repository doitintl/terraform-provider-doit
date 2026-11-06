package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// rejectNullCollectionElements adds NoNullValues to every Required or Optional
// list and set of primitives, including those nested in other attributes.
// ValueStringsAre and UniqueValues skip null elements, so `ids = [null]`
// otherwise passes plan and fails at apply when a request builder converts the
// collection with ElementsAs into a non-pointer slice. The API has no
// representation for a null element, so rejecting it is always correct.
func rejectNullCollectionElements(attributes map[string]schema.Attribute) {
	for name, attribute := range attributes {
		switch a := attribute.(type) {
		case schema.ListAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				a.Validators = append(a.Validators, listvalidator.NoNullValues())
				attributes[name] = a
			}
		case schema.SetAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				a.Validators = append(a.Validators, setvalidator.NoNullValues())
				attributes[name] = a
			}
		case schema.SingleNestedAttribute:
			rejectNullCollectionElements(a.Attributes)
		case schema.ListNestedAttribute:
			rejectNullCollectionElements(a.NestedObject.Attributes)
		case schema.SetNestedAttribute:
			rejectNullCollectionElements(a.NestedObject.Attributes)
		case schema.MapNestedAttribute:
			rejectNullCollectionElements(a.NestedObject.Attributes)
		default:
			// Scalars, maps and objects have no elements that ElementsAs
			// converts into a non-pointer slice.
		}
	}
}

// rejectNullDataSourceCollectionElements is rejectNullCollectionElements for
// data source schemas.
func rejectNullDataSourceCollectionElements(attributes map[string]dsschema.Attribute) {
	for name, attribute := range attributes {
		switch a := attribute.(type) {
		case dsschema.ListAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				a.Validators = append(a.Validators, listvalidator.NoNullValues())
				attributes[name] = a
			}
		case dsschema.SetAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				a.Validators = append(a.Validators, setvalidator.NoNullValues())
				attributes[name] = a
			}
		case dsschema.SingleNestedAttribute:
			rejectNullDataSourceCollectionElements(a.Attributes)
		case dsschema.ListNestedAttribute:
			rejectNullDataSourceCollectionElements(a.NestedObject.Attributes)
		case dsschema.SetNestedAttribute:
			rejectNullDataSourceCollectionElements(a.NestedObject.Attributes)
		case dsschema.MapNestedAttribute:
			rejectNullDataSourceCollectionElements(a.NestedObject.Attributes)
		default:
			// Scalars, maps and objects have no elements that ElementsAs
			// converts into a non-pointer slice.
		}
	}
}

func configurable(required, optional bool) bool {
	return required || optional
}

func isPrimitiveElement(t attr.Type) bool {
	switch t {
	case types.StringType, types.Int64Type, types.Float64Type, types.NumberType, types.BoolType:
		return true
	}
	return false
}
