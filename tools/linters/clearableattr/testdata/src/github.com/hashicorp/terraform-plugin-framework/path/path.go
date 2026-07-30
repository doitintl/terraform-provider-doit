// Stub package for analysistest.
package path

// Path is a stub for the terraform-plugin-framework path.Path type.
type Path struct{}

// Root is a stub for path.Root.
func Root(string) Path { return Path{} }

// AtName is a stub for Path.AtName.
func (Path) AtName(string) Path { return Path{} }
