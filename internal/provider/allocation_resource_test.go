package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAllocation(t *testing.T) {
	n := rand.Int()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationSingle(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			{
				Config: testAccAllocationSingleUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.this",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("single")),
				},
			},
			{
				ResourceName:      "doit_allocation.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAllocation_Group(t *testing.T) {
	n := rand.Int()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroup(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.group",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.group",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("group")),
				},
			},
			{
				ResourceName:      "doit_allocation.group",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"description",
					"rules",
				},
			},
		},
	})
}

func TestAccAllocation_Validation(t *testing.T) {
	n := rand.Int()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationValidation(n),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func testAccAllocationSingle(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name = "test-%d"
	description = "test allocation"
    rule = {
       formula = "A AND B"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["JP"]
         },
         {
           key    = "project_id"
           mode   = "is"
           type   = "fixed"
           values = ["test-k8s-project-468707"]
          }
       ]
    }
}
`, i)
}

func testAccAllocationSingleUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name = "test-%d"
	description = "test allocation updated"
    rule = {
       formula = "A AND B"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["JP"]
         },
         {
           key    = "project_id"
           mode   = "is"
           type   = "fixed"
           values = ["test-k8s-project-468707"]
          }
       ]
    }
}
`, i)
}

func testAccAllocationGroup(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name = "test-group-%d"
	description = "test allocation group"
    rules = [
        {
            action = "create"
       formula = "A AND B"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["JP"]
         },
         {
           key    = "project_id"
           mode   = "is"
           type   = "fixed"
           values = ["test-k8s-project-468707"]
          }
       ]
    },
           {
            action = "create"
       formula = "A AND B"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["US"]
         },
         {
           key    = "project_id"
           mode   = "is"
           type   = "fixed"
           values = ["test-k8s-project-468707"]
          }
       ]
    }
    ]
}
`, i)
}

func testAccAllocationValidation(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "validation" {
    name = "test-validation-%d"
	description = "test allocation validation"
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
    }
    rules = [
        {
            action = "create"
            formula = "A"
            components = [
                {
                    key    = "country"
                    mode   = "is"
                    type   = "fixed"
                    values = ["US"]
                }
            ]
        }
    ]
}
`, i)
}
