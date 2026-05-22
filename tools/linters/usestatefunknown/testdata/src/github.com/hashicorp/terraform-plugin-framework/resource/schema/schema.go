// Stub package for analysistest.
package schema

import "context"

type Schema struct {
	Attributes          map[string]Attribute
	Description         string
	MarkdownDescription string
}

type Attribute interface{}

type StringAttribute struct {
	Required            bool
	Optional            bool
	Computed            bool
	PlanModifiers       []interface{}
	Description         string
	MarkdownDescription string
}

type Float64Attribute struct {
	Required            bool
	Optional            bool
	Computed            bool
	Description         string
	MarkdownDescription string
}

type BoolAttribute struct {
	Required            bool
	Optional            bool
	Computed            bool
	Description         string
	MarkdownDescription string
}

type ListAttribute struct {
	ElementType         interface{}
	Required            bool
	Optional            bool
	Computed            bool
	Description         string
	MarkdownDescription string
}

type ListNestedAttribute struct {
	NestedObject        NestedAttributeObject
	Required            bool
	Optional            bool
	Computed            bool
	Description         string
	MarkdownDescription string
}

type NestedAttributeObject struct {
	Attributes map[string]Attribute
}

type Int64Attribute struct {
	Required            bool
	Optional            bool
	Computed            bool
	PlanModifiers       []interface{}
	Description         string
	MarkdownDescription string
}

type SingleNestedAttribute struct {
	Attributes          map[string]Attribute
	Required            bool
	Optional            bool
	Computed            bool
	Description         string
	MarkdownDescription string
}

var _ context.Context
