package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccCustomMetricDataSource_Basic reads a metric with display precision and
// one without by ID, then checks that a second apply plans nothing.
func TestAccCustomMetricDataSource_Basic(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tfacc-cm")

	config := testAccCustomMetricsDataSourceMetrics(prefix, `
data "doit_custom_metric" "with_decimals" {
  id = doit_custom_metric.m1.id
}

data "doit_custom_metric" "without_decimals" {
  id = doit_custom_metric.m2.id
}
`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("name"), knownvalue.StringExact(prefix+"-1")),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("description"), knownvalue.StringExact("first")),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("formula"), knownvalue.StringExact("A / B")),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("unit_type"), knownvalue.StringExact("percentage")),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("owner"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("etag"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("create_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("update_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("decimals"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"min": knownvalue.Int64Exact(0),
						"max": knownvalue.Int64Exact(2),
					})),
					statecheck.ExpectKnownValue("data.doit_custom_metric.with_decimals", tfjsonpath.New("variables"), knownvalue.ListExact([]knownvalue.Check{
						knownVariable("A", "basic", "cost"),
						knownVariable("B", "basic", "usage"),
					})),
					statecheck.ExpectKnownValue("data.doit_custom_metric.without_decimals", tfjsonpath.New("decimals"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_custom_metric.without_decimals", tfjsonpath.New("variables"), knownvalue.ListExact([]knownvalue.Check{
						knownVariable("A", "basic", "cost"),
					})),
				},
			},
			expectEmptyPlanStep(config),
		},
	})
}

// TestAccCustomMetricDataSource_NotFound reads an ID that no custom metric has.
func TestAccCustomMetricDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_custom_metric" "missing" {
  id = "tfacc-does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`(?s)Error Reading Custom Metric.*404`),
			},
		},
	})
}

// TestAccCustomMetricDataSource_UnknownInput reads a metric whose ID is only
// known after apply: the read waits until the metric exists.
func TestAccCustomMetricDataSource_UnknownInput(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tfacc-cm")

	config := testAccCustomMetricsDataSourceMetrics(prefix, `
data "doit_custom_metric" "m3" {
  id = doit_custom_metric.m3.id
}

output "m3_name" {
  value = data.doit_custom_metric.m3.name
}
`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("m3_name", knownvalue.StringExact(prefix+"-3")),
				},
			},
		},
	})
}
