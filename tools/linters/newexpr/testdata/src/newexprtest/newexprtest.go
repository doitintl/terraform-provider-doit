package newexprtest

type params struct {
	Filter *string
	SortBy *string
	Name   *string
}

type data struct{}

func (d data) ValueString() string { return "" }

// BAD: temp var then take address — method call
func badTempVar() {
	var p params
	var d data
	filter := d.ValueString() // want `use new\(d.ValueString\(\.\.\.\)\) instead of temp variable "filter" then &filter`
	p.Filter = &filter
	_ = p
}

// BAD: temp var with string literal
func badTempVar2() {
	var p params
	name := "hello" // want `use new\(expr\) instead of temp variable "name" then &name`
	p.Name = &name
	_ = p
}

// GOOD: variable used elsewhere (not just &x)
func goodUsedElsewhere() {
	var p params
	var d data
	filter := d.ValueString()
	_ = filter // used here
	p.Filter = &filter
	_ = p
}

// GOOD: already using new(expr) or direct pointer
func goodDirect() {
	var p params
	p.Filter = new(string)
	_ = p
}

// GOOD: composite literal (&type{} is a constructor, excluded)
type myStruct struct{}

func goodCompositeLit() {
	s := myStruct{}
	_ = &s
}
