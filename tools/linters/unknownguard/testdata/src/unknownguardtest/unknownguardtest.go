package unknownguardtest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"unknownguardtest/datasource_test"
)

type model struct{}

func (m model) IsUnknown() bool { return false }

type testDataSource struct{}

func (d *testDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_test.TestDataSourceSchema(ctx)
	resp.Schema = s
}

// BAD: Read() with Required input but no unknown guard
func (d *testDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) { // want `data source Read\(\) must check for unknown inputs`
	// Directly makes API call without checking for unknown inputs
}

type goodDataSource struct{}

func (d *goodDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_test.TestDataSourceSchema(ctx)
	resp.Schema = s
}

// GOOD: Read() checks for unknown inputs
func (d *goodDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data model
	if data.IsUnknown() {
		return
	}
}
