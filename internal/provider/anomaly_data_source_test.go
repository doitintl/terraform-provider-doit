package provider_test

import (
	"fmt"
	"os"
	"regexp"
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
					resource.TestCheckResourceAttrSet("data.doit_anomaly.test", "notifications.#"),
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

// TestAccAnomalyDataSource_DeactivationReason verifies that the
// deactivation_reason attribute is mapped to a non-null enum value.
// TEST_ANOMALY_ID is a years-old anomaly in the test tenant that is always
// deactivated (the tenant has no anomalies that stay reliably active), so
// this asserts the actual mapped value rather than just checking presence.
func TestAccAnomalyDataSource_DeactivationReason(t *testing.T) {
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
				Config: testAccAnomalyDataSourceDeactivationReasonConfig(anomalyID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomaly.test", "id", anomalyID),
					resource.TestMatchOutput("deactivation_reason", regexp.MustCompile(`^(reverted|expired|unknown)$`)),
				),
			},
			// Drift verification
			{
				Config: testAccAnomalyDataSourceDeactivationReasonConfig(anomalyID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAnomalyDataSourceDeactivationReasonConfig(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly" "test" {
  id = %[1]q
}

output "deactivation_reason" {
  value = data.doit_anomaly.test.deactivation_reason
}
`, id)
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

output "notifications" {
  value = [for n in data.doit_anomaly.test.notifications : n.channel]
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

// TestAccAnomalyDataSource_CostFields verifies that the actual_cost and
// expected_max_cost attributes are accessible.
func TestAccAnomalyDataSource_CostFields(t *testing.T) {
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
				Config: testAccAnomalyDataSourceCostConfig(anomalyID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomaly.test", "id", anomalyID),
				),
			},
			// Drift verification
			{
				Config: testAccAnomalyDataSourceCostConfig(anomalyID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAnomalyDataSourceCostConfig(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly" "test" {
  id = %[1]q
}

output "actual_cost" {
  value = data.doit_anomaly.test.actual_cost != null ? data.doit_anomaly.test.actual_cost : 0.0
}

output "expected_max_cost" {
  value = data.doit_anomaly.test.expected_max_cost != null ? data.doit_anomaly.test.expected_max_cost : 0.0
}
`, id)
}
