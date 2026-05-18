package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccAnomalyDataSource_Basic(t *testing.T) {
	anomalyID := os.Getenv("TEST_ANOMALY_ID")
	if anomalyID == "" {
		t.Skip("TEST_ANOMALY_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomalyDataSourceConfig(anomalyID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomaly.test", "id", anomalyID),
					resource.TestCheckResourceAttrSet("data.doit_anomaly.test", "platform"),
					resource.TestCheckResourceAttrSet("data.doit_anomaly.test", "service_name"),
					resource.TestCheckResourceAttrSet("data.doit_anomaly.test", "resource_data.#"),
					// acknowledged is always a bool (true/false), never null
					resource.TestCheckResourceAttrSet("data.doit_anomaly.test", "acknowledged"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAnomalyDataSourceConfig(anomalyID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAnomalyDataSource_ResourceDataLabels verifies that the labels nested
// attribute inside resource_data is populated correctly (may be empty list for
// anomalies without cost-allocation tags).
func TestAccAnomalyDataSource_ResourceDataLabels(t *testing.T) {
	anomalyID := os.Getenv("TEST_ANOMALY_ID")
	if anomalyID == "" {
		t.Skip("TEST_ANOMALY_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomalyDataSourceConfigWithLabelsOutput(anomalyID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomaly.test", "id", anomalyID),
					// Verify resource_data exists and each item has a labels attribute
					resource.TestCheckResourceAttrSet("data.doit_anomaly.test", "resource_data.#"),
				),
			},
			// Drift verification
			{
				Config: testAccAnomalyDataSourceConfigWithLabelsOutput(anomalyID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAnomalyDataSource_AcknowledgedFields verifies that the acknowledged_at
// and acknowledged_by attributes are accessible. These may be null when the
// anomaly has not been acknowledged, so we use output expressions to validate
// the mapping works without requiring specific values.
func TestAccAnomalyDataSource_AcknowledgedFields(t *testing.T) {
	anomalyID := os.Getenv("TEST_ANOMALY_ID")
	if anomalyID == "" {
		t.Skip("TEST_ANOMALY_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomalyDataSourceAcknowledgedConfig(anomalyID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomaly.test", "id", anomalyID),
					resource.TestCheckResourceAttrSet("data.doit_anomaly.test", "acknowledged"),
				),
			},
			// Drift verification
			{
				Config: testAccAnomalyDataSourceAcknowledgedConfig(anomalyID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAnomalyDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly" "test" {
  id = %[1]q
}
`, id)
}

func testAccAnomalyDataSourceAcknowledgedConfig(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly" "test" {
  id = %[1]q
}

output "acknowledged" {
  value = data.doit_anomaly.test.acknowledged
}

output "acknowledged_at" {
  value = data.doit_anomaly.test.acknowledged_at
}

output "acknowledged_by" {
  value = data.doit_anomaly.test.acknowledged_by
}
`, id)
}

func testAccAnomalyDataSourceConfigWithLabelsOutput(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly" "test" {
  id = %[1]q
}

output "resource_labels" {
  value = [for rd in data.doit_anomaly.test.resource_data : rd.labels]
}
`, id)
}
