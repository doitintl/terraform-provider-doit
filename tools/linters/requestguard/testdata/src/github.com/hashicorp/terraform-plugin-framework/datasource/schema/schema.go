// Stub package for analysistest (datasource schema).
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
	Default             interface{}
	Description         string
	MarkdownDescription string
}

var _ context.Context
