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

// BAD: temp var inside if block
func badInsideIf() {
	var p params
	var d data
	if true {
		filter := d.ValueString() // want `use new\(d.ValueString\(\.\.\.\)\) instead of temp variable "filter" then \&filter`
		p.Filter = &filter
	}
	_ = p
}

// BAD: temp var inside for loop
func badInsideFor() {
	var p params
	var d data
	for i := 0; i < 1; i++ {
		sortBy := d.ValueString() // want `use new\(d.ValueString\(\.\.\.\)\) instead of temp variable "sortBy" then \&sortBy`
		p.SortBy = &sortBy
	}
	_ = p
}

// BAD: temp var inside range loop
func badInsideRange() {
	var p params
	for range []int{1} {
		name := "hello" // want `use new\(expr\) instead of temp variable "name" then \&name`
		p.Name = &name
	}
	_ = p
}

// BAD: temp var in deeply nested blocks
func badDeeplyNested() {
	var p params
	var d data
	if true {
		if true {
			filter := d.ValueString() // want `use new\(d.ValueString\(\.\.\.\)\) instead of temp variable "filter" then \&filter`
			p.Filter = &filter
		}
	}
	_ = p
}

// GOOD: variable used elsewhere inside nested block
func goodUsedElsewhereNested() {
	var p params
	var d data
	if true {
		filter := d.ValueString()
		_ = filter // used here
		p.Filter = &filter
	}
	_ = p
}

// BAD: temp var in else block
func badInsideElse() {
	var p params
	var d data
	if false {
		_ = d
	} else {
		filter := d.ValueString() // want `use new\(d.ValueString\(\.\.\.\)\) instead of temp variable "filter" then \&filter`
		p.Filter = &filter
	}
	_ = p
}

// BAD: temp var in switch case
func badInsideSwitch(x int) {
	var p params
	switch x {
	case 1:
		name := "hello" // want `use new\(expr\) instead of temp variable "name" then \&name`
		p.Name = &name
	}
	_ = p
}

// BAD: temp var in type switch case
func badInsideTypeSwitch(v any) {
	var p params
	switch v.(type) {
	case string:
		name := "typed" // want `use new\(expr\) instead of temp variable "name" then \&name`
		p.Name = &name
	}
	_ = p
}
