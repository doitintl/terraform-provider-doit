package unknownguardtest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"unknownguardtest/datasource_list"
)

type value struct{}

func (v value) IsNull() bool    { return false }
func (v value) IsUnknown() bool { return false }

func StringUnknown() value { return value{} }
func Int64Unknown() value  { return value{} }

type listModel struct {
	Filter    value
	PageToken value
	RowCount  value
}

type raw struct{}

func (raw) IsFullyKnown() bool { return true }

type config struct{ Raw raw }

type listRequest struct{ Config config }

func listConfig(datasource.ReadRequest) listRequest { return listRequest{} }

// GOOD: IsFullyKnown guard that sets both results unknown unconditionally.
type goodListDataSource struct{}

func (d *goodListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *goodListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data listModel
	if !listConfig(req).Config.Raw.IsFullyKnown() {
		data.RowCount = Int64Unknown()
		data.PageToken = StringUnknown()
		return
	}
}

// BAD: a per-field guard, which misses unknown values it does not list.
type perFieldListDataSource struct{}

func (d *perFieldListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *perFieldListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) { // want `list data source Read\(\) must guard unknown protocol configuration with req.Config.Raw.IsFullyKnown\(\)`
	var data listModel
	if data.Filter.IsUnknown() || data.PageToken.IsUnknown() {
		data.RowCount = Int64Unknown()
		data.PageToken = StringUnknown()
		return
	}
}

// BAD: page_token set unknown only when it is null, so a configured token
// survives although a read replaces it.
type conditionalListDataSource struct{}

func (d *conditionalListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *conditionalListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data listModel
	if !listConfig(req).Config.Raw.IsFullyKnown() { // want `list data source unknown guard must set PageToken = types.StringUnknown\(\) and RowCount = types.Int64Unknown\(\) unconditionally`
		data.RowCount = Int64Unknown()
		if data.PageToken.IsNull() {
			data.PageToken = StringUnknown()
		}
		return
	}
}

// BAD: row_count is left as it was.
type noRowCountListDataSource struct{}

func (d *noRowCountListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *noRowCountListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data listModel
	if !listConfig(req).Config.Raw.IsFullyKnown() { // want `list data source unknown guard must set PageToken`
		data.PageToken = StringUnknown()
		return
	}
}

func (v value) IsKnown() bool { return true }

type maxResultsModel struct {
	listModel
	MaxResults value
}

// BAD: the guard only covers auto-pagination mode, so unknown configuration
// with max_results set reaches the API.
type restrictedListDataSource struct{}

func (d *restrictedListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *restrictedListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) { // want `list data source Read\(\) must guard unknown protocol configuration with req.Config.Raw.IsFullyKnown\(\)`
	var data maxResultsModel
	if !listConfig(req).Config.Raw.IsFullyKnown() && data.MaxResults.IsNull() {
		data.RowCount = Int64Unknown()
		data.PageToken = StringUnknown()
		return
	}
}

// BAD: the condition is inverted, so the fallback runs for known configuration.
type invertedListDataSource struct{}

func (d *invertedListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *invertedListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) { // want `list data source Read\(\) must guard unknown protocol configuration with req.Config.Raw.IsFullyKnown\(\)`
	var data listModel
	if listConfig(req).Config.Raw.IsFullyKnown() {
		data.RowCount = Int64Unknown()
		data.PageToken = StringUnknown()
		return
	}
}

// GOOD: a guard that also fires on other conditions still covers every
// configuration that is not fully known.
type widerListDataSource struct{}

func (d *widerListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *widerListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data maxResultsModel
	if !listConfig(req).Config.Raw.IsFullyKnown() || !data.MaxResults.IsKnown() {
		data.RowCount = Int64Unknown()
		data.PageToken = StringUnknown()
		return
	}
}

// BAD: the guard sets the results unknown but does not return, so the read
// goes on to call the API with unknown configuration.
type noReturnListDataSource struct{}

func (d *noReturnListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *noReturnListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data listModel
	if !listConfig(req).Config.Raw.IsFullyKnown() { // want `list data source unknown guard must end with return`
		data.RowCount = Int64Unknown()
		data.PageToken = StringUnknown()
	}
}

// BAD: the guard is nested in a branch, so unknown configuration with
// max_results set skips it.
type nestedListDataSource struct{}

func (d *nestedListDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_list.ListDataSourceSchema(ctx)
	resp.Schema = s
}

func (d *nestedListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) { // want `list data source Read\(\) must guard unknown protocol configuration with req.Config.Raw.IsFullyKnown\(\)`
	var data maxResultsModel
	if data.MaxResults.IsNull() {
		if !listConfig(req).Config.Raw.IsFullyKnown() {
			data.RowCount = Int64Unknown()
			data.PageToken = StringUnknown()
			return
		}
	}
}
