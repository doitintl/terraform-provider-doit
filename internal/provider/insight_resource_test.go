package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccInsightResource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccInsightResourceConfig(rName, "Basic Test Insight", "A basic test insight"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_insight.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("key"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("Basic Test Insight")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("A basic test insight")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("cloud_provider"),
						knownvalue.StringExact("aws")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("categories"),
						knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("resource_results"),
						knownvalue.ListSizeExact(1)),
				},
			},
			// Step 2: Drift check — re-apply same config, expect no changes
			{
				Config: testAccInsightResourceConfig(rName, "Basic Test Insight", "A basic test insight"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccInsightResource_Update(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccInsightResourceConfig(rName, "Original Title", "Original description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("Original Title")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("Original description")),
				},
			},
			// Step 2: Update title and description
			{
				Config: testAccInsightResourceConfig(rName, "Updated Title", "Updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("Updated Title")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("Updated description")),
				},
			},
			// Step 3: Drift check
			{
				Config: testAccInsightResourceConfig(rName, "Updated Title", "Updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccInsightResource_ImportState(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccInsightResourceConfig(rName, "Import Test Insight", "Test import functionality"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("key"),
						knownvalue.StringExact(rName)),
				},
			},
			// Step 2: Import
			{
				ResourceName:                         "doit_insight.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("public-api/%s", rName),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "key",
				// resource_results come from a separate API endpoint; timeouts are client-only
				ImportStateVerifyIgnore: []string{"resource_results", "timeouts"},
			},
			// Step 3: Drift check — re-apply config after import, expect no changes
			{
				Config: testAccInsightResourceConfig(rName, "Import Test Insight", "Test import functionality"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightResourceConfig(key, title, description string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = %[2]q
  short_description = %[3]q
  cloud_provider    = "aws"
  categories        = ["FinOps"]

  resource_results = [{
    resource_id    = "i-acc-test-001"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"

    result = {
      value = 5.42
    }
  }]
}
`, key, title, description)
}
