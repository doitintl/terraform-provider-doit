// Resource file with CORRECT configure type string.
package good_resource

import "context"

type configureRequest struct{ ProviderData interface{} }
type configureResponse struct{ Diagnostics diagList }
type diagList struct{}
func (d diagList) AddError(summary, detail string) {}

type myResource struct{}

func (r *myResource) Configure(_ context.Context, req configureRequest, resp *configureResponse) {
	if req.ProviderData == nil {
		return
	}
	_, ok := req.ProviderData.(*myResource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type", // Correct for a resource ✓
			"Expected *myResource",
		)
	}
}
