package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccWidgetDataSource_Spend(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWidgetDataSourceConfig("current-month-cloud-spend"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_widget.test", "alias", "current-month-cloud-spend"),
					resource.TestCheckResourceAttr("data.doit_widget.test", "type", "preset"),
					resource.TestCheckResourceAttr("data.doit_widget.test", "kind", "monetary_metric"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "generate_time"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.value.amount"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.value.currency"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.period.start_time"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.period.end_time"),
				),
			},
			{
				Config: testAccWidgetDataSourceConfig("current-month-cloud-spend"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWidgetDataSource_Forecast(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWidgetDataSourceConfig("current-month-cloud-forecast"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_widget.test", "alias", "current-month-cloud-forecast"),
					resource.TestCheckResourceAttr("data.doit_widget.test", "type", "preset"),
					resource.TestCheckResourceAttr("data.doit_widget.test", "kind", "monetary_metric"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "generate_time"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.value.amount"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.value.currency"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.period.start_time"),
					resource.TestCheckResourceAttrSet("data.doit_widget.test", "result.monetary_metric.period.end_time"),
				),
			},
			{
				Config: testAccWidgetDataSourceConfig("current-month-cloud-forecast"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWidgetDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccWidgetDataSourceConfig("nonexistent-widget-xyz-999"),
				ExpectError: regexp.MustCompile(`(?i)(not found|404)`),
			},
		},
	})
}

func testAccWidgetDataSourceConfig(widgetID string) string {
	return fmt.Sprintf(`
data "doit_widget" "test" {
  widget_id = %[1]q
}
`, widgetID)
}
