package provider_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAllocPrefix is the namespace prefix for all test allocation names.
// The sweep function deletes any allocation whose name starts with this prefix.
const testAllocPrefix = "tfacc"

func testProject() string {
	return os.Getenv("TEST_PROJECT")
}

func TestAccAllocation(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationSingle(rName),
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
				Config: testAccAllocationSingleUpdate(rName),
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
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroup(rName),
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
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationValidation(rName),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestAccAllocation_Group_Select(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupSelect(rName),
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
			// Step 2: Re-apply - verify no drift.
			// Tests that components handling for "select" rules doesn't cause
			// an inconsistent result (user omits components, API may return null/empty).
			{
				Config: testAccAllocationGroupSelect(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
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
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationGroupMissingUnallocatedCosts(rName),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestAccAllocation_UnallocatedCosts_ConflictsWithRule(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationSingleWithUnallocatedCosts(rName),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

// --- Config generators ---

func testAccAllocationSingle(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name = "%s"
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
`, rName, testProject())
}

func testAccAllocationSingleUpdate(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name = "%s"
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
`, rName, testProject())
}

func testAccAllocationGroup(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name = "%s-group"
	description = "test allocation group"
    unallocated_costs = "%s-other"
    rules = [
        {
            action = "create"
            name   = "%s-jp-rule"
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
            name   = "%s-us-rule"
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
`, rName, rName, rName, testProject(), rName, testProject())
}

func testAccAllocationValidation(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "validation" {
    name = "%s-validation"
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
`, rName)
}

func testAccAllocationGroupSelect(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name = "%s-source"
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
    name = "%s-group-select"
	description = "test allocation group select"
    unallocated_costs = "%s-other"
    rules = [
        {
            action = "select"
            id     = doit_allocation.this.id
        }
    ]
}
`, rName, rName, rName)
}

func testAccAllocationGroupMissingUnallocatedCosts(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group_missing_unallocated" {
    name = "%s-group-missing"
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
`, rName)
}

func testAccAllocationSingleWithUnallocatedCosts(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "single_with_unallocated" {
    name = "%s-single-unallocated"
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
`, rName)
}

// TestAccAllocation_MissingNameInCreateAction tests that rules with action="create"
// are rejected at plan time if they don't have a "name" field.
func TestAccAllocation_MissingNameInCreateAction(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationGroupMissingName(rName),
				ExpectError: regexp.MustCompile(`'name' is required when action is 'create'`),
			},
		},
	})
}

func testAccAllocationGroupMissingName(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "missing_name" {
    name = "%s-missing-name"
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
`, rName)
}

// TestAccAllocation_ComponentFlags tests the include_null and inverse_selection
// boolean flags on allocation rule components.
func TestAccAllocation_ComponentFlags(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationComponentFlags(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_allocation.flags", "id"),
					resource.TestCheckResourceAttr("doit_allocation.flags", "rules.0.components.0.include_null", "true"),
					resource.TestCheckResourceAttr("doit_allocation.flags", "rules.0.components.0.inverse_selection", "true"),
				),
			},
			// Verify no drift on re-apply
			{
				Config: testAccAllocationComponentFlags(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationComponentFlags(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "flags" {
    name = "%s-flags"
    description = "Test allocation with component flags"
    unallocated_costs = "%s-other"
    rules = [
        {
            action = "create"
            name   = "%s-flag-rule"
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
`, rName, rName, rName)
}

// TestAccAllocation_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccAllocation_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccAllocationSingle(rName),
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
				Config:             testAccAllocationSingle(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}

// TestAccAllocation_ListAttributes_EmptyRules tests that an empty rules list
// is blocked by the validator. The provider correctly handles empty lists
// (returns empty list even when API returns nil), but the validator guides
// users toward omitting the attribute instead, which is cleaner.
func TestAccAllocation_ListAttributes_EmptyRules(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationWithEmptyRules(rName),
				ExpectError: regexp.MustCompile(`Invalid Rules Configuration|rules cannot be empty`),
			},
		},
	})
}

func testAccAllocationWithEmptyRules(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name        = "%s-empty-rules"
    description = "test allocation with empty rules"
    unallocated_costs = "Other"
    rules = []
}
`, rName)
}

// TestAccAllocation_ListAttributes_EmptyComponents tests that an empty components list
// in a rule is rejected - the formula validation fails without valid components.
func TestAccAllocation_ListAttributes_EmptyComponents(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationWithEmptyComponents(rName),
				ExpectError: regexp.MustCompile(`formula validation failed|components|required|empty`),
			},
		},
	})
}

func testAccAllocationWithEmptyComponents(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name        = "%s-empty-components"
    description = "test allocation with empty components"
    rule = {
       formula = "A"
       components = []
    }
}
`, rName)
}

// TestAccAllocation_ListAttributes_OmittedRulesAndRule tests that omitting both
// rule and rules produces an error.
func TestAccAllocation_ListAttributes_OmittedRulesAndRule(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationWithOmittedRulesAndRule(rName),
				ExpectError: regexp.MustCompile(`rule|required|expected`),
			},
		},
	})
}

func testAccAllocationWithOmittedRulesAndRule(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name        = "%s-no-rules"
    description = "test allocation with no rule or rules"
    # Both rule and rules omitted - should fail
}
`, rName)
}

// TestAccAllocation_NestedAllocationRule tests that an allocation can reference
// another allocation using the "allocation_rule" dimension type. This creates
// a base allocation, then a second allocation whose rule references the first.
func TestAccAllocation_NestedAllocationRule(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationNestedRule(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.nested",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.nested",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("single")),
				},
			},
			// Step 2: Re-apply - verify no drift
			{
				Config: testAccAllocationNestedRule(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "doit_allocation.nested",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"update_time", // Computed field that changes on each modification
				},
			},
		},
	})
}

func testAccAllocationNestedRule(rName string) string {
	return fmt.Sprintf(`
# Base allocation that will be referenced by the nested allocation
resource "doit_allocation" "base" {
    name        = "%s-base"
    description = "base allocation for nesting test"
    rule = {
       formula = "A"
       components = [
        {
           key    = "project_id"
           mode   = "is"
           type   = "fixed"
           values = ["%s"]
         }
       ]
    }
}

# Nested allocation that references the base allocation using allocation_rule type
resource "doit_allocation" "nested" {
    name        = "%s-nested"
    description = "nested allocation referencing base"
    rule = {
       formula = "A"
       components = [
        {
           key    = "allocation_rule"
           mode   = "is"
           type   = "allocation_rule"
           values = [doit_allocation.base.id]
         }
       ]
    }
}
`, rName, testProject(), rName)
}

// TestAccAllocation_NestedAllocationRule_InvalidMode tests that using an unsupported
// mode with type="allocation_rule" is rejected at plan time by the validator.
func TestAccAllocation_NestedAllocationRule_InvalidMode(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationNestedRuleInvalidMode(rName),
				ExpectError: regexp.MustCompile(`Invalid Allocation Rule Component`),
			},
		},
	})
}

// testAccCheckAllocationDestroy returns a CheckDestroy function that:
// 1. Verifies all Terraform-managed doit_allocation resources return 404
// 2. Sweeps orphaned allocations whose names start with testAllocPrefix
//
// The sweep is needed because group allocations create child "single" allocations
// for each rule, and the API does NOT cascade-delete them when the parent is deleted.
func testAccCheckAllocationDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getAPIClient(t)
		ctx := context.Background()

		// 1. Verify Terraform-managed allocations are deleted
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "doit_allocation" {
				continue
			}
			resp, err := client.GetAllocationWithResponse(ctx, rs.Primary.ID)
			if err != nil {
				return fmt.Errorf("error checking allocation %s: %w", rs.Primary.ID, err)
			}
			if resp.StatusCode() != 404 {
				return fmt.Errorf("allocation %s still exists (status %d)", rs.Primary.ID, resp.StatusCode())
			}
		}

		// 2. Sweep orphaned test allocations (child rule allocations from group tests)
		sweepTestAllocations(t, client)

		return nil
	}
}

// sweepTestAllocations lists all allocations and deletes any whose name starts
// with testAllocPrefix. This catches orphaned rule allocations that the API creates
// for group allocation rules but does not cascade-delete.
func sweepTestAllocations(t *testing.T, client *models.ClientWithResponses) {
	t.Helper()
	ctx := context.Background()

	sweepPrefix := testAllocPrefix + "-"
	var toDelete []string
	params := &models.ListAllocationsParams{}

	for {
		resp, err := client.ListAllocationsWithResponse(ctx, params)
		if err != nil {
			t.Logf("Warning: failed to list allocations for sweep: %v", err)
			return
		}
		if resp.JSON200 == nil || resp.JSON200.Allocations == nil {
			break
		}

		for _, a := range *resp.JSON200.Allocations {
			if a.Name != nil && strings.HasPrefix(*a.Name, sweepPrefix) {
				toDelete = append(toDelete, *a.Id)
			}
		}

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}

	for _, id := range toDelete {
		resp, err := client.DeleteAllocationWithResponse(ctx, id)
		if err != nil {
			t.Logf("Warning: failed to delete orphaned allocation %s: %v", id, err)
			continue
		}
		if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
			t.Logf("Warning: unexpected status %d deleting orphaned allocation %s", resp.StatusCode(), id)
		}
	}

	if len(toDelete) > 0 {
		t.Logf("Swept %d orphaned test allocation(s)", len(toDelete))
	}
}

func testAccAllocationNestedRuleInvalidMode(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "invalid_nested" {
    name        = "%s-invalid-nested"
    description = "nested allocation with invalid mode"
    rule = {
       formula = "A"
       components = [
        {
           key    = "allocation_rule"
           mode   = "starts_with"
           type   = "allocation_rule"
           values = ["some-allocation-id"]
         }
       ]
    }
}
`, rName)
}
