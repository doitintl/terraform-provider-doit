package interfacetest

type MyInterface interface {
	DoSomething()
}

type myType struct{}

func (m *myType) DoSomething() {}

type anotherType struct{}

func (a *anotherType) DoSomething() {}

// BAD: uses &type{}
var _ MyInterface = &myType{} // want `use \(\*myType\)\(nil\) instead of &myType\{\}`

// GOOD: uses (*type)(nil)
var _ MyInterface = (*anotherType)(nil)
