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

// TestAccAllocation_GroupUpdate_OmittedRuleDescriptionKept updates one rule of a
// group allocation and omits the description of another rule that has one stored.
// The omitted description plans unknown and the request builder sends the
// unknown as an empty string, which could clear the stored value.
func TestAccAllocation_GroupUpdate_OmittedRuleDescriptionKept(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	config := func(action, jpDescription, usCountry string) string {
		return fmt.Sprintf(`
resource "doit_allocation" "group" {
  name              = "%[1]s-group"
  description       = "test allocation group"
  unallocated_costs = "%[1]s-other"
  rules = [
    {
      action      = %[2]q
      name        = "%[1]s-jp-rule"
      formula     = "A AND B"
      %[3]s
      components = [
        { key = "country", mode = "is", type = "fixed", values = ["JP"] },
        { key = "project_id", mode = "is", type = "fixed", values = ["%[5]s"] },
      ]
    },
    {
      action  = %[2]q
      name    = "%[1]s-us-rule"
      formula = "A AND B"
      components = [
        { key = "country", mode = "is", type = "fixed", values = ["%[4]s"] },
        { key = "project_id", mode = "is", type = "fixed", values = ["%[5]s"] },
      ]
    },
  ]
}
`, rName, action, jpDescription, usCountry, testProject())
	}
	descriptionPath := tfjsonpath.New("rules").AtSliceIndex(0).AtMapKey("description")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config("create", `description = "keep me"`, "US"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_allocation.group", descriptionPath, knownvalue.StringExact("keep me")),
				},
			},
			{
				Config: config("update", "", "FR"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectUnknownValue("doit_allocation.group", descriptionPath),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_allocation.group", descriptionPath, knownvalue.StringExact("keep me")),
				},
			},
		},
	})
}
