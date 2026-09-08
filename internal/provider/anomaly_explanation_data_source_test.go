package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAnomalyExplanationDataSource_Basic verifies that the anomaly explanation
// data source maps the deterministic facts, evidence references, and the
// AI-generated explanation. The explanation.text field is non-deterministic
// (re-generated per call), so only presence is asserted for the AI-generated
// content; ai_generated is asserted to be true per the API contract.
func TestAccAnomalyExplanationDataSource_Basic(t *testing.T) {
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
				Config: testAccAnomalyExplanationDataSourceConfig(anomalyID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomaly_explanation.test", "id", anomalyID),
					// facts (deterministic)
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "facts.service_name"),
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "facts.platform"),
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "facts.scope"),
					resource.TestMatchResourceAttr("data.doit_anomaly_explanation.test", "facts.severity_level", regexp.MustCompile(`^(information|warning|critical)$`)),
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "facts.cost_of_anomaly"),
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "facts.top3skus.#"),
					// evidence (list of references)
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "evidence.#"),
					// explanation (AI-generated; assert presence, not content)
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "explanation.text"),
					resource.TestCheckResourceAttrSet("data.doit_anomaly_explanation.test", "explanation.generated_by"),
					resource.TestCheckResourceAttr("data.doit_anomaly_explanation.test", "explanation.ai_generated", "true"),
				),
			},
			// NOTE: No drift check step. The explanation.text field is
			// AI-generated and non-deterministic — re-reading during a drift
			// step would produce a diff on every run. See the doit_ava data
			// source for the same precedent.
		},
	})
}

// TestAccAnomalyExplanationDataSource_NullableCostFields verifies that the
// nullable actual_cost and expected_max_cost attributes are accessible. These
// may be null when the API omits them, so an output expression with a fallback
// validates the mapping without requiring specific values.
func TestAccAnomalyExplanationDataSource_NullableCostFields(t *testing.T) {
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
				Config: testAccAnomalyExplanationNullableCostConfig(anomalyID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomaly_explanation.test", "id", anomalyID),
				),
			},
			// No drift step (non-deterministic explanation.text).
		},
	})
}

func testAccAnomalyExplanationDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly_explanation" "test" {
  id = %[1]q
}
`, id)
}

func testAccAnomalyExplanationNullableCostConfig(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly_explanation" "test" {
  id = %[1]q
}

output "actual_cost" {
  value = data.doit_anomaly_explanation.test.facts.actual_cost != null ? data.doit_anomaly_explanation.test.facts.actual_cost : 0.0
}

output "expected_max_cost" {
  value = data.doit_anomaly_explanation.test.facts.expected_max_cost != null ? data.doit_anomaly_explanation.test.facts.expected_max_cost : 0.0
}
`, id)
}
