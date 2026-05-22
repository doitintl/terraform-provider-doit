package structtest

import "resource_budget"

// --- BAD: struct literal with fields ---

func badStructLiteral() {
	_ = resource_budget.ScopesValue{ // want "do not construct resource_budget.ScopesValue with struct literal"
		Id:   "foo",
		Mode: "is",
	}
}

func badAlertsLiteral() {
	_ = resource_budget.AlertsValue{ // want "do not construct resource_budget.AlertsValue with struct literal"
		Percentage: 0.5,
		Triggered:  true,
	}
}

// --- GOOD: empty struct literal (used as method receiver) ---

func goodEmptyLiteral() {
	_ = resource_budget.ScopesValue{} // OK - empty literal for method calls
}

func goodEmptyAlerts() {
	_ = resource_budget.AlertsValue{} // OK - empty literal
}

// --- GOOD: non-generated package struct literals are fine ---

type localValue struct {
	Name string
}

func goodLocalStructLiteral() {
	_ = localValue{Name: "test"} // OK - not from resource_* package
}
