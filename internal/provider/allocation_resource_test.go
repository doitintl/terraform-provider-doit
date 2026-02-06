package provider_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func testProject() string {
	return os.Getenv("TEST_PROJECT")
}

func TestAccAllocation(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

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
				ImportStateVerifyIgnore: []string{
					"update_time", // Computed field that changes on each modification
				},
			},
		},
	})
}

func TestAccAllocation_Group(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

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
					"update_time", // Computed field that changes on each modification
					"rules",       // API doesn't return the 'action' field for rules
				},
			},
		},
	})
}

func TestAccAllocation_Validation(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

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

func TestAccAllocation_Group_Select(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupSelect(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.group_select",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.group_select",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("group")),
				},
			},
			{
				ResourceName:      "doit_allocation.group_select",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"update_time", // Computed field that changes on each modification
					"rules",       // API doesn't return the 'action' field for rules
				},
			},
		},
	})
}

func TestAccAllocation_UnallocatedCosts_RequiredWithRules(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationGroupMissingUnallocatedCosts(n),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestAccAllocation_UnallocatedCosts_ConflictsWithRule(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationSingleWithUnallocatedCosts(n),
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
           values = ["%s"]
          }
       ]
    }
}
`, i, testProject())
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
           values = ["%s"]
          }
       ]
    }
}
`, i, testProject())
}

func testAccAllocationGroup(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name = "test-group-%d"
	description = "test allocation group"
    unallocated_costs = "Other"
    rules = [
        {
            action = "create"
            name   = "JP Rule"
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
           values = ["%s"]
          }
       ]
    },
           {
            action = "create"
            name   = "US Rule"
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
           values = ["%s"]
          }
       ]
    }
    ]
}
`, i, testProject(), testProject())
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

func testAccAllocationGroupSelect(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name = "test-source-%d"
	description = "test allocation source"
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
}

resource "doit_allocation" "group_select" {
    name = "test-group-select-%d"
	description = "test allocation group select"
    unallocated_costs = "Other"
    rules = [
        {
            action = "select"
            id     = doit_allocation.this.id
        }
    ]
}
`, i, i)
}

func testAccAllocationGroupMissingUnallocatedCosts(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group_missing_unallocated" {
    name = "test-group-missing-%d"
	description = "test allocation group missing unallocated_costs"
    rules = [
        {
            action = "create"
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
    ]
}
`, i)
}

func testAccAllocationSingleWithUnallocatedCosts(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "single_with_unallocated" {
    name = "test-single-unallocated-%d"
	description = "test allocation single with unallocated_costs"
    unallocated_costs = "Other"
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
}
`, i)
}

// TestAccAllocation_MissingNameInCreateAction tests that rules with action="create"
// are rejected at plan time if they don't have a "name" field.
func TestAccAllocation_MissingNameInCreateAction(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationGroupMissingName(n),
				ExpectError: regexp.MustCompile(`'name' is required when action is 'create'`),
			},
		},
	})
}

func testAccAllocationGroupMissingName(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "missing_name" {
    name = "test-missing-name-%d"
	description = "test allocation group missing name in rule"
    unallocated_costs = "Other"
    rules = [
        {
            action = "create"
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
    ]
}
`, i)
}

// TestAccAllocation_ComponentFlags tests the include_null and inverse_selection
// boolean flags on allocation rule components.
func TestAccAllocation_ComponentFlags(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationComponentFlags(n),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_allocation.flags", "id"),
					resource.TestCheckResourceAttr("doit_allocation.flags", "rules.0.components.0.include_null", "true"),
					resource.TestCheckResourceAttr("doit_allocation.flags", "rules.0.components.0.inverse_selection", "true"),
				),
			},
			// Verify no drift on re-apply
			{
				Config: testAccAllocationComponentFlags(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationComponentFlags(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "flags" {
    name = "test-flags-%d"
    description = "Test allocation with component flags"
    unallocated_costs = "Other"
    rules = [
        {
            action = "create"
            name   = "Flag Test Rule"
            formula = "A"
            components = [
                {
                    key               = "country"
                    mode              = "is"
                    type              = "fixed"
                    values            = ["JP"]
                    include_null      = true
                    inverse_selection = true
                }
            ]
        }
    ]
}
`, i)
}

// TestAccAllocation_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccAllocation_Disappears(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	var resourceId string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccAllocationSingle(n),
				Check: resource.ComposeTestCheckFunc(
					// Capture the resource ID for later deletion
					resource.TestCheckResourceAttrWith("doit_allocation.this", "id", func(value string) error {
						if value == "" {
							return fmt.Errorf("resource ID is empty")
						}
						resourceId = value
						return nil
					}),
				),
			},
			// Step 2: Delete the resource via API, then verify Terraform detects the drift
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteAllocationWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete allocation via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204, or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccAllocationSingle(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}
