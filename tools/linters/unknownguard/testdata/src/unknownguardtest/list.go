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
