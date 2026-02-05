// Package tfplugingenframework provides code generation tools for Terraform plugin framework.
package tfplugingenframework

//go:generate go run github.com/doitintl/terraform-plugin-codegen-framework/cmd/tfplugingen-framework generate resources --input output_resources.json --output ../../internal/provider
//go:generate go run github.com/doitintl/terraform-plugin-codegen-framework/cmd/tfplugingen-framework generate data-sources --input output_datasources.json --output ../../internal/provider
