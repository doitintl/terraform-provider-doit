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

// testAccAllocationSingleValueExtraction renders a single allocation whose rule carries
// the given value_extraction block (omitted when veBlock is empty).
func testAccAllocationSingleValueExtraction(rName, description, veBlock string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
  name        = %[1]q
  description = %[2]q
  rule = {
    formula = "A"
    components = [
      {
        key    = "country"
        mode   = "is"
        type   = "fixed"
        values = ["JP"]
      }
    ]
%[3]s  }
}
`, rName, description, veBlock)
}

func testAccSingleValueExtractionBlock(key, fallback string) string {
	return fmt.Sprintf(`    value_extraction = {
      fallback = %q
      sources = [
        {
          type = "tag"
          key  = %q
        },
      ]
    }
`, fallback, key)
}

// Category A lifecycle for rule.value_extraction on a single allocation: create, keep
// across an unrelated update (CMP-56890: the first update used to erase it), change,
// clear by removing the block, and clear together with a sibling change.
func TestAccAllocation_SingleValueExtraction(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	const res = "doit_allocation.this"
	ve := tfjsonpath.New("rule").AtMapKey("value_extraction")

	withEnv := testAccSingleValueExtractionBlock("Environment", "Unknown")
	withTeam := testAccSingleValueExtractionBlock("Team", "Unknown")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create with value_extraction.
			{
				Config: testAccAllocationSingleValueExtraction(rName, "initial", withEnv),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, ve.AtMapKey("fallback"), knownvalue.StringExact("Unknown")),
					statecheck.ExpectKnownValue(res, ve.AtMapKey("on_missing"), knownvalue.StringExact("useFallback")),
					statecheck.ExpectKnownValue(res, ve.AtMapKey("sources").AtSliceIndex(0).AtMapKey("key"), knownvalue.StringExact("Environment")),
				},
			},
			// Step 2: drift check.
			{
				Config:           testAccAllocationSingleValueExtraction(rName, "initial", withEnv),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// Step 3: an update that changes only the description keeps value_extraction.
			{
				Config: testAccAllocationSingleValueExtraction(rName, "updated description", withEnv),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, tfjsonpath.New("description"), knownvalue.StringExact("updated description")),
					statecheck.ExpectKnownValue(res, ve.AtMapKey("sources").AtSliceIndex(0).AtMapKey("key"), knownvalue.StringExact("Environment")),
				},
			},
			// Step 4: drift check after the description-only update.
			{
				Config:           testAccAllocationSingleValueExtraction(rName, "updated description", withEnv),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// Step 5: replace the source.
			{
				Config: testAccAllocationSingleValueExtraction(rName, "updated description", withTeam),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, ve.AtMapKey("sources").AtSliceIndex(0).AtMapKey("key"), knownvalue.StringExact("Team")),
				},
			},
			// Step 6: import round-trips the block.
			{
				ResourceName:            res,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"update_time"},
			},
			// Step 7: removing the block clears it (explicit null on the wire).
			{
				Config: testAccAllocationSingleValueExtraction(rName, "updated description", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, ve, knownvalue.Null()),
				},
			},
			// Step 8: drift check after clearing.
			{
				Config:           testAccAllocationSingleValueExtraction(rName, "updated description", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// Step 9: set it again.
			{
				Config: testAccAllocationSingleValueExtraction(rName, "updated description", withEnv),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, ve.AtMapKey("sources").AtSliceIndex(0).AtMapKey("key"), knownvalue.StringExact("Environment")),
				},
			},
			// Step 10: remove the block and change a sibling in the same apply.
			{
				Config: testAccAllocationSingleValueExtraction(rName, "cleared with sibling change", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, ve, knownvalue.Null()),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("description"), knownvalue.StringExact("cleared with sibling change")),
				},
			},
			// Step 11: drift check after the combined clear.
			{
				Config:           testAccAllocationSingleValueExtraction(rName, "cleared with sibling change", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// The data source exposes rule.value_extraction of a single allocation.
func TestAccAllocationDataSource_Single_ValueExtraction(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alloc-ds-ve")
	config := testAccAllocationSingleValueExtraction(rName, "ds", testAccSingleValueExtractionBlock("Environment", "DefaultEnv")) + `
data "doit_allocation" "test" {
  id = doit_allocation.this.id
}
`

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_allocation.test",
						tfjsonpath.New("rule").AtMapKey("value_extraction").AtMapKey("fallback"),
						knownvalue.StringExact("DefaultEnv")),
					statecheck.ExpectKnownValue("data.doit_allocation.test",
						tfjsonpath.New("rule").AtMapKey("value_extraction").AtMapKey("on_missing"),
						knownvalue.StringExact("useFallback")),
					statecheck.ExpectKnownValue("data.doit_allocation.test",
						tfjsonpath.New("rule").AtMapKey("value_extraction").AtMapKey("sources").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("Environment")),
				},
			},
		},
	})
}
