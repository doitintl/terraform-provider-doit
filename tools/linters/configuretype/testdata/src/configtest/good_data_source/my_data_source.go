// Data source file with correct configure type string.
package good_data_source

import "context"

type configureRequest struct{ ProviderData interface{} }
type configureResponse struct{ Diagnostics diagList }
type diagList struct{}
func (d diagList) AddError(summary, detail string) {}

type myDataSource struct{}

func (d *myDataSource) Configure(_ context.Context, req configureRequest, resp *configureResponse) {
	if req.ProviderData == nil {
		return
	}
	_, ok := req.ProviderData.(*myDataSource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type", // Correct for a data source ✓
			"Expected *myDataSource",
		)
	}
}
