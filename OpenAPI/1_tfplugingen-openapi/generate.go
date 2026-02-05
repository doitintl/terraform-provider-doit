// Package tfplugingenopenapi provides code generation tools for OpenAPI spec processing.
package tfplugingenopenapi

//go:generate go run github.com/doitintl/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi generate -config resources.yml --output ../2_tfplugingen-framework/output_resources.json ../openapi_spec_full.yml
//go:generate go run github.com/doitintl/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi generate -config datasources.yml --output ../2_tfplugingen-framework/output_datasources.json ../openapi_spec_full.yml
