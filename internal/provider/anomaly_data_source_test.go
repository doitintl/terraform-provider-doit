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

func testAccAnomalyDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_anomaly" "test" {
  id = %[1]q
}
`, id)
}
