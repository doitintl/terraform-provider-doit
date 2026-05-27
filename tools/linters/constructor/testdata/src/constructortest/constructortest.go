package constructortest

type Resource interface{}
type DataSource interface{}

type myResource struct{}
type myDataSource struct{}

// BAD: uses new(type) instead of &type{}
func NewBadResource() Resource {
	return new(myResource) // want `constructor NewBadResource should return &type\{\} instead of new\(type\)`
}

// GOOD: uses &type{} style
func NewGoodResource() Resource {
	return &myResource{}
}

// GOOD: data source with &type{}
func NewGoodDataSource() DataSource {
	return &myDataSource{}
}

// Not a constructor — should not be flagged
func NewHelper() *myResource {
	return new(myResource)
}
