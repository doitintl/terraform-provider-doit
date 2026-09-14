package attr

type Value interface {
	IsNull() bool
	IsUnknown() bool
}
