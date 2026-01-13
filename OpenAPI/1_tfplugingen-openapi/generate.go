// Package tfplugingenopenapi provides code generation tools for OpenAPI spec processing.
package tfplugingenopenapi

//go:generate go run github.com/hashicorp/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi generate -config resources.yml --output ../2_tfplugingen-framework/output.json ../openapi_spec_full.yml
