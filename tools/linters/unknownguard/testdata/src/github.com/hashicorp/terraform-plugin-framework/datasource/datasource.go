package datasource

import "context"

type ReadRequest struct{}
type ReadResponse struct{}
type SchemaRequest struct{}
type SchemaResponse struct{ Schema interface{} }
type ConfigureRequest struct{}
type ConfigureResponse struct{}
type MetadataRequest struct{}
type MetadataResponse struct{ TypeName string }
type DataSource interface{}
type DataSourceWithConfigure interface{}
type ReadFunc func(ctx context.Context, req ReadRequest, resp *ReadResponse)
