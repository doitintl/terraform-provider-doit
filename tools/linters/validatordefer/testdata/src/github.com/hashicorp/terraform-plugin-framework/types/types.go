package types

type String struct {
	unknown bool
	null    bool
	value   string
}

func StringUnknown() String {
	return String{unknown: true}
}

func (value String) IsUnknown() bool {
	return value.unknown
}

func (value String) IsNull() bool {
	return value.null
}

func (value String) ValueString() string {
	return value.value
}
