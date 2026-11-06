package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestResourceCollectionsRejectNullElements fails when a resource schema has a
// user-configurable list or set of primitives that accepts a null element.
// New resources must add NoNullValues (see rejectNullCollectionElements).
func TestResourceCollectionsRejectNullElements(t *testing.T) {
	ctx := t.Context()
	p := &doitProvider{}
	for _, newResource := range p.Resources(ctx) {
		r := newResource()
		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "doit"}, &meta)

		var resp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &resp)

		t.Run(meta.TypeName, func(t *testing.T) {
			checkNullElementRejection(ctx, t, "", resp.Schema.Attributes)
		})
	}
}

// TestDataSourceCollectionsRejectNullElements is the data source counterpart of
// TestResourceCollectionsRejectNullElements.
func TestDataSourceCollectionsRejectNullElements(t *testing.T) {
	ctx := t.Context()
	p := &doitProvider{}
	for _, newDataSource := range p.DataSources(ctx) {
		d := newDataSource()
		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "doit"}, &meta)

		var resp datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &resp)

		t.Run(meta.TypeName, func(t *testing.T) {
			checkDataSourceNullElementRejection(ctx, t, "", resp.Schema.Attributes)
		})
	}
}

func checkDataSourceNullElementRejection(ctx context.Context, t *testing.T, prefix string, attributes map[string]dsschema.Attribute) {
	t.Helper()
	for name, attribute := range attributes {
		at := prefix + name
		switch a := attribute.(type) {
		case dsschema.ListAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				requireNullElementRejected(ctx, t, at, types.ListValueMust(a.ElementType, []attr.Value{nullOf(a.ElementType)}), a.Validators)
			}
		case dsschema.SetAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				requireNullElementRejected(ctx, t, at, types.SetValueMust(a.ElementType, []attr.Value{nullOf(a.ElementType)}), a.Validators)
			}
		case dsschema.SingleNestedAttribute:
			checkDataSourceNullElementRejection(ctx, t, at+".", a.Attributes)
		case dsschema.ListNestedAttribute:
			checkDataSourceNullElementRejection(ctx, t, at+"[*].", a.NestedObject.Attributes)
		case dsschema.SetNestedAttribute:
			checkDataSourceNullElementRejection(ctx, t, at+"[*].", a.NestedObject.Attributes)
		case dsschema.MapNestedAttribute:
			checkDataSourceNullElementRejection(ctx, t, at+"[*].", a.NestedObject.Attributes)
		}
	}
}

func checkNullElementRejection(ctx context.Context, t *testing.T, prefix string, attributes map[string]schema.Attribute) {
	t.Helper()
	for name, attribute := range attributes {
		at := prefix + name
		switch a := attribute.(type) {
		case schema.ListAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				requireNullElementRejected(ctx, t, at, types.ListValueMust(a.ElementType, []attr.Value{nullOf(a.ElementType)}), a.Validators)
			}
		case schema.SetAttribute:
			if configurable(a.Required, a.Optional) && isPrimitiveElement(a.ElementType) {
				requireNullElementRejected(ctx, t, at, types.SetValueMust(a.ElementType, []attr.Value{nullOf(a.ElementType)}), a.Validators)
			}
		case schema.SingleNestedAttribute:
			checkNullElementRejection(ctx, t, at+".", a.Attributes)
		case schema.ListNestedAttribute:
			checkNullElementRejection(ctx, t, at+"[*].", a.NestedObject.Attributes)
		case schema.SetNestedAttribute:
			checkNullElementRejection(ctx, t, at+"[*].", a.NestedObject.Attributes)
		case schema.MapNestedAttribute:
			checkNullElementRejection(ctx, t, at+"[*].", a.NestedObject.Attributes)
		}
	}
}

func nullOf(t attr.Type) attr.Value {
	switch t {
	case types.Int64Type:
		return types.Int64Null()
	case types.Float64Type:
		return types.Float64Null()
	case types.NumberType:
		return types.NumberNull()
	case types.BoolType:
		return types.BoolNull()
	}
	return types.StringNull()
}

func requireNullElementRejected(ctx context.Context, t *testing.T, at string, value attr.Value, validators any) {
	t.Helper()
	rejected := false
	switch v := value.(type) {
	case types.List:
		for _, val := range validators.([]validator.List) {
			var resp validator.ListResponse
			val.ValidateList(ctx, validator.ListRequest{ConfigValue: v}, &resp)
			rejected = rejected || resp.Diagnostics.HasError()
		}
	case types.Set:
		for _, val := range validators.([]validator.Set) {
			var resp validator.SetResponse
			val.ValidateSet(ctx, validator.SetRequest{ConfigValue: v}, &resp)
			rejected = rejected || resp.Diagnostics.HasError()
		}
	}
	if !rejected {
		t.Errorf("%s accepts a null element; add NoNullValues (see rejectNullCollectionElements)", at)
	}
}
