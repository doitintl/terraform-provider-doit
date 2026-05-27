package schema

type Schema struct {
	Attributes map[string]Attribute
}
type Attribute interface{}
type StringAttribute struct {
	Required bool
	Optional bool
	Computed bool
}
