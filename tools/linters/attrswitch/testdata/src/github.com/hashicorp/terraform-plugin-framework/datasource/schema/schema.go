// Stub package for analysistest.
package schema

type Attribute interface{ IsAttribute() }

type PlanModifier interface{}

type Validator interface{}

type StringAttribute struct {
	PlanModifiers []PlanModifier
	Validators    []Validator
	Default       interface{}
}

type Int64Attribute struct {
	PlanModifiers []PlanModifier
	Validators    []Validator
	Default       interface{}
}

type NestedAttributeObject struct {
	Attributes map[string]Attribute
}

type SingleNestedAttribute struct {
	Attributes    map[string]Attribute
	PlanModifiers []PlanModifier
	Validators    []Validator
}

type ListNestedAttribute struct {
	NestedObject  NestedAttributeObject
	PlanModifiers []PlanModifier
	Validators    []Validator
}

type SetNestedAttribute struct {
	NestedObject  NestedAttributeObject
	PlanModifiers []PlanModifier
	Validators    []Validator
}

type MapNestedAttribute struct {
	NestedObject  NestedAttributeObject
	PlanModifiers []PlanModifier
	Validators    []Validator
}

func (StringAttribute) IsAttribute()       {}
func (Int64Attribute) IsAttribute()        {}
func (SingleNestedAttribute) IsAttribute() {}
func (ListNestedAttribute) IsAttribute()   {}
func (SetNestedAttribute) IsAttribute()    {}
func (MapNestedAttribute) IsAttribute()    {}
