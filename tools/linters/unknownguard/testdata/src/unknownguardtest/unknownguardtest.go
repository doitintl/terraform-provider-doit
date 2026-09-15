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

// BAD: Read() with Required input but no defensive protocol guard
func (d *testDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) { // want `data source Read\(\) must defensively guard unknown protocol configuration`
	// Directly makes an API call without guarding unknown protocol configuration.
}

type goodDataSource struct{}

func (d *goodDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_test.TestDataSourceSchema(ctx)
	resp.Schema = s
}

// GOOD: Read() defensively checks for unknown protocol configuration
func (d *goodDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data model
	if data.IsUnknown() {
		return
	}
}
