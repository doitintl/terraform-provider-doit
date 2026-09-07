package provider_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccReportQueryDataSource_Async(t *testing.T) {
	config := strings.Replace(testAccReportQueryDataSourceConfig(), `data "doit_report_query" "test" {`, "data \"doit_report_query\" \"test\" {\n async = true\n timeouts = { read = \"15m\" }", 1)
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t), TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.doit_report_query.test", "result_json"),
			resource.TestCheckResourceAttrSet("data.doit_report_query.test", "row_count"),
			resource.TestCheckResourceAttrSet("data.doit_report_query.test", "cache_hit"),
		)}, {Config: config}},
	})
}

func TestAccReportResultDataSource_Async(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-async")
	config := strings.Replace(testAccReportResultDataSourceWithDateRangeConfig(name), `data "doit_report_result" "test" {`, "data \"doit_report_result\" \"test\" {\n async = true\n timeouts = { read = \"15m\" }", 1)
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t), TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.doit_report_result.test", "result_json"),
			resource.TestCheckResourceAttr("data.doit_report_result.test", "report_name", name),
			resource.TestCheckResourceAttr("data.doit_report_result.test", "start_date", "2026-01-01"),
			resource.TestCheckResourceAttr("data.doit_report_result.test", "end_date", "2026-01-31"),
		)}, {Config: config}},
	})
}
