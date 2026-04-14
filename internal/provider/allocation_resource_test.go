package provider_test

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"testing"

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
			// Drift detection: re-apply same config, expect no changes.
			{
				Config: testAccAllocationSingle(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
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
			// Drift detection: re-apply same config, expect no changes.
			{
				Config: testAccAllocationGroup(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
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

// TestAccAllocation_CaseInsensitive tests the case_insensitive flag on
// allocation rule components. Requires mode=contains|starts_with|ends_with.
func TestAccAllocation_CaseInsensitive(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationCaseInsensitive(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_allocation.ci", "id"),
					resource.TestCheckResourceAttr("doit_allocation.ci", "rules.0.components.0.case_insensitive", "true"),
					resource.TestCheckResourceAttr("doit_allocation.ci", "rules.0.components.0.mode", "contains"),
				),
			},
			// Verify no drift on re-apply
			{
				Config: testAccAllocationCaseInsensitive(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationCaseInsensitive(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "ci" {
    name = "%s-ci"
    description = "Test allocation with case-insensitive filter"
    unallocated_costs = "%s-other"
    rules = [
        {
            action = "create"
            name   = "%s-ci-rule"
            formula = "A"
            components = [
                {
                    key              = "country"
                    mode             = "contains"
                    type             = "fixed"
                    values           = ["jp"]
                    case_insensitive = true
                }
            ]
        }
    ]
}
`, rName, rName, rName)
}

// TestAccAllocation_InverseField tests the new "inverse" attribute on allocation
// rule components. This is separate from TestAccAllocation_ComponentFlags which
// tests the legacy "inverse_selection" attribute for backward compatibility.
func TestAccAllocation_InverseField(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationInverseField(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_allocation.inverse", "id"),
					resource.TestCheckResourceAttr("doit_allocation.inverse", "rules.0.components.0.inverse", "true"),
				),
			},
			// Verify no drift on re-apply
			{
				Config: testAccAllocationInverseField(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationInverseField(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inverse" {
    name = "%s-inverse"
    description = "Test allocation with inverse field"
    unallocated_costs = "%s-other"
    rules = [
        {
            action = "create"
            name   = "%s-inverse-rule"
            formula = "A"
            components = [
                {
                    key     = "country"
                    mode    = "is"
                    type    = "fixed"
                    values  = ["JP"]
                    inverse = true
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

// testAccCheckAllocationDestroy returns a CheckDestroy function that verifies
// Terraform-managed allocations have been deleted. It does NOT sweep orphaned
// allocations — that is handled once by TestMain after all tests complete,
// to avoid deleting allocations that belong to other parallel tests.
func testAccCheckAllocationDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getAPIClient(t)
		ctx := context.Background()

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

		return nil
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

// TestAccAllocation_InverseMigration tests that migrating a single allocation
// from the deprecated "inverse_selection" attribute to the new "inverse"
// attribute does not produce "inconsistent result" errors.
//
// The API internally maps "inverse" back to "inverse_selection" in the response,
// so the response always has inverse_selection=true and inverse=false. The
// provider must preserve the user's planned values in state rather than
// writing the API's swapped values.
func TestAccAllocation_InverseMigration(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with the legacy inverse_selection = true.
			{
				Config: testAccAllocationInverseSelectionLegacy(rName),
			},
			// Step 2: Migrate to inverse = true (remove inverse_selection).
			// On v1.3.3, this crashes with "inconsistent result" because
			// the API maps inverse back to inverse_selection.
			{
				Config: testAccAllocationInverseMigrated(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.inv_migrate",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"inverse": knownvalue.Bool(true),
							}),
						}),
					),
				},
			},
			// Step 3: Re-apply — verify no drift.
			{
				Config: testAccAllocationInverseMigrated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationInverseSelectionLegacy(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inv_migrate" {
    name        = "%s-inv-migrate"
    description = "test allocation with inverse_selection migration"
    rule = {
       formula = "A"
       components = [
        {
           key               = "service_description"
           mode              = "is"
           type              = "fixed"
           values            = ["AmazonCloudWatch"]
           inverse_selection = true
         }
       ]
    }
}
`, rName)
}

func testAccAllocationInverseMigrated(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inv_migrate" {
    name        = "%s-inv-migrate"
    description = "test allocation with inverse_selection migration"
    rule = {
       formula = "A"
       components = [
        {
           key     = "service_description"
           mode    = "is"
           type    = "fixed"
           values  = ["AmazonCloudWatch"]
           inverse = true
         }
       ]
    }
}
`, rName)
}

// TestAccAllocation_SentinelRestore tests that a single allocation with a
// NullFallback sentinel value (e.g. "[Label N/A]") in component values
// round-trips correctly — after apply the state still contains "[Label N/A]"
// and no "inconsistent result" error is produced.
//
// The API strips NullFallback sentinels from the values list and returns
// values=[] with include_null=true. The provider must preserve the user's
// planned values in state to avoid a list-length mismatch.
func TestAccAllocation_SentinelRestore(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with "[Label N/A]" sentinel. Must not crash
			// with "inconsistent result".
			{
				Config: testAccAllocationSentinelOnly(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.sentinel",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.sentinel",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"include_null": knownvalue.Bool(true),
								// The sentinel must be preserved in state.
								"values": knownvalue.ListExact([]knownvalue.Check{
									knownvalue.StringExact("[Service N/A]"),
								}),
							}),
						}),
					),
				},
			},
			// Step 2: Re-apply — verify no drift.
			{
				Config: testAccAllocationSentinelOnly(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationSentinelOnly(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "sentinel" {
    name        = "%s-sentinel"
    description = "test allocation with sentinel value"
    rule = {
       formula = "A"
       components = [
        {
           key          = "service_description"
           mode         = "is"
           type         = "fixed"
           include_null = true
           values       = ["[Service N/A]"]
         }
       ]
    }
}
`, rName)
}

// TestAccAllocation_SentinelMixed tests that a single allocation with BOTH a
// NullFallback sentinel AND real values round-trips without "inconsistent result".
//
// The API strips the sentinel from the values list and sets include_null=true.
// If the provider writes the API response (which has N-1 values) to state,
// Terraform sees each value shifted by one index and crashes. The provider
// must preserve the user's planned values in state.
func TestAccAllocation_SentinelMixed(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationSentinelMixed(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.sentinel_mixed",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.sentinel_mixed",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"include_null": knownvalue.Bool(true),
								"values": knownvalue.ListExact([]knownvalue.Check{
									// Both must be preserved in state.
									knownvalue.StringExact("[Service N/A]"),
									knownvalue.StringExact("AmazonCloudWatch"),
								}),
							}),
						}),
					),
				},
			},
			// Step 2: Re-apply — verify no drift.
			{
				Config: testAccAllocationSentinelMixed(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationSentinelMixed(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "sentinel_mixed" {
    name        = "%s-sentinel-mixed"
    description = "test allocation with sentinel and real values"
    rule = {
       formula = "A"
       components = [
        {
           key          = "service_description"
           mode         = "is"
           type         = "fixed"
           include_null = true
           values       = ["[Service N/A]", "AmazonCloudWatch"]
         }
       ]
    }
}
`, rName)
}

// TestAccAllocation_ValueNormalization tests that a single allocation with a
// service_description value that the API normalizes does not crash with
// "inconsistent result".
//
// The API normalizes some service names (e.g. stripping the "(EKS)" suffix from
// "Amazon Elastic Container Service for Kubernetes (EKS)"). The provider must
// preserve the user's planned values in state after Create/Update. The Read path
// then detects the normalized value as drift and surfaces it in the next plan.
//
// Asserts:
//   - Step 1: Create succeeds without crash; drift is detected (non-empty plan)
//   - Step 2: Using the canonical name produces an empty plan (no drift)
func TestAccAllocation_ValueNormalization(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with the non-canonical name.
			// The plan-first pattern prevents "inconsistent result" crash.
			// The API normalizes the value, so the post-apply refresh will
			// write the canonical name to state, causing expected drift.
			{
				Config:             testAccAllocationValueNormalization(rName),
				ExpectNonEmptyPlan: true, // Expected: Read returns API's canonical name → drift.
			},
			// Step 2: Switch to the API's canonical name.
			// State already has the canonical name from step 1's refresh,
			// so this should produce no drift.
			{
				Config: testAccAllocationValueNormalizationCanonical(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationValueNormalization(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "norm" {
    name        = "%s-value-norm"
    description = "test allocation with API-normalized service name"
    rule = {
       formula = "A"
       components = [
        {
           key    = "service_description"
           mode   = "is"
           type   = "fixed"
           values = ["Amazon Elastic Container Service for Kubernetes (EKS)"]
         }
       ]
    }
}
`, rName)
}

func testAccAllocationValueNormalizationCanonical(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "norm" {
    name        = "%s-value-norm"
    description = "test allocation with API-normalized service name"
    rule = {
       formula = "A"
       components = [
        {
           key    = "service_description"
           mode   = "is"
           type   = "fixed"
           values = ["Amazon Elastic Container Service for Kubernetes"]
         }
       ]
    }
}
`, rName)
}

// ---------------------------------------------------------------------------
// Coverage Audit Gap Tests (added to close all blind spots)
// ---------------------------------------------------------------------------

// TestAccAllocation_GroupUpdate tests updating a group allocation: changing rule
// names, values, and components. This exercises the plan-first state pattern for
// group allocations during Update, including action preservation and rule ID matching.
func TestAccAllocation_GroupUpdate(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create a group allocation with two rules.
			{
				Config: testAccAllocationGroup(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.group",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("group")),
				},
			},
			// Step 2: Update — change country value from "US" to "DE" in the second rule.
			{
				Config: testAccAllocationGroupUpdated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.group",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Step 3: Drift check — re-apply, expect no changes.
			{
				Config: testAccAllocationGroupUpdated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupUpdated(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name = "%s-group"
	description = "test allocation group updated"
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
            name   = "%s-de-rule"
       formula = "A AND B"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["DE"]
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

// TestAccAllocation_GroupOmittedDefaults tests creating a group allocation where
// the Optional+Computed field "description" is omitted from the config.
// The provider must correctly resolve the unknown to the API default to avoid
// perpetual drift.
func TestAccAllocation_GroupOmittedDefaults(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with description omitted from the rule.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "defaults" {
    name = "%s-defaults"
    description = "test omitted defaults"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-rule"
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
`, rName, rName, rName),
			},
			// Step 2: Drift check — verify no drift from the API's defaults.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "defaults" {
    name = "%s-defaults"
    description = "test omitted defaults"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-rule"
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
`, rName, rName, rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAllocation_SingleMinimal tests creating a single allocation with the
// absolute minimum required fields, exercising the overlay code's handling of
// all Optional+Computed fields defaulting to unknown.
func TestAccAllocation_SingleMinimal(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with only required fields, omitting all optional booleans.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "minimal" {
    name = "%s-minimal"
    description = "test minimal config"
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
`, rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.minimal",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"case_insensitive":  knownvalue.Bool(false),
								"include_null":      knownvalue.Bool(false),
								"inverse":           knownvalue.Bool(false),
								"inverse_selection": knownvalue.Bool(false),
							}),
						}),
					),
				},
			},
			// Step 2: Drift check.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "minimal" {
    name = "%s-minimal"
    description = "test minimal config"
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
`, rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAllocation_DimensionsTypeAlias tests that the DimensionsType alias
// normalization preserves the user's configured type when the API returns
// the canonical equivalent. The alias pair tested: allocation_rule ↔ attribution.
func TestAccAllocation_DimensionsTypeAlias(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create a single-rule allocation with type "allocation_rule".
			// The API returns the canonical "attribution" in responses.
			// The normalizer should preserve "allocation_rule" in state.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "alias_src" {
    name = "%s-alias-src"
    description = "source allocation for alias test"
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

resource "doit_allocation" "alias_test" {
    name = "%s-alias"
    description = "test dimensions type alias"
    rule = {
       formula = "A"
       components = [
        {
           key    = "allocation_rule"
           mode   = "is"
           type   = "allocation_rule"
           values = [doit_allocation.alias_src.id]
         }
       ]
    }
}
`, rName, rName),
			},
			// Step 2: Drift check — verify the alias type is preserved.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "alias_src" {
    name = "%s-alias-src"
    description = "source allocation for alias test"
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

resource "doit_allocation" "alias_test" {
    name = "%s-alias"
    description = "test dimensions type alias"
    rule = {
       formula = "A"
       components = [
        {
           key    = "allocation_rule"
           mode   = "is"
           type   = "allocation_rule"
           values = [doit_allocation.alias_src.id]
         }
       ]
    }
}
`, rName, rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAllocation_AddRemoveComponents tests adding and removing components
// to an existing allocation during an update. This exercises the index-based
// state matching in the Read path (sentinel merge, boolean flag preservation).
func TestAccAllocation_AddRemoveComponents(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with 1 component.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "comp_change" {
    name = "%s-comp-change"
    description = "test add/remove components"
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
`, rName),
			},
			// Step 2: Update to 2 components — add project_id.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "comp_change" {
    name = "%s-comp-change"
    description = "test add/remove components"
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
`, rName, testProject()),
			},
			// Step 3: Drift check after adding component.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "comp_change" {
    name = "%s-comp-change"
    description = "test add/remove components"
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
`, rName, testProject()),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 4: Back to 1 component — remove project_id.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "comp_change" {
    name = "%s-comp-change"
    description = "test add/remove components"
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
`, rName),
			},
			// Step 5: Drift check after removing component.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "comp_change" {
    name = "%s-comp-change"
    description = "test add/remove components"
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
`, rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAllocation_MultipleValues tests creating a component with multiple
// values in the values list. This exercises the values list mapping more
// thoroughly than single-value tests.
func TestAccAllocation_MultipleValues(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "multi_val" {
    name = "%s-multi-val"
    description = "test multiple values"
    rule = {
       formula = "A"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["JP", "US", "DE", "FR"]
         }
       ]
    }
}
`, rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.multi_val",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"values": knownvalue.ListExact([]knownvalue.Check{
									knownvalue.StringExact("JP"),
									knownvalue.StringExact("US"),
									knownvalue.StringExact("DE"),
									knownvalue.StringExact("FR"),
								}),
							}),
						}),
					),
				},
			},
			// Drift check.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "multi_val" {
    name = "%s-multi-val"
    description = "test multiple values"
    rule = {
       formula = "A"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["JP", "US", "DE", "FR"]
         }
       ]
    }
}
`, rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAllocation_IncludeNullEmptyValues tests creating a component with
// include_null=true and an empty values list. This verifies the base case
// for null-inclusion without sentinel values.
func TestAccAllocation_IncludeNullEmptyValues(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "incl_null" {
    name = "%s-incl-null"
    description = "test include_null with empty values"
    rule = {
       formula = "A"
       components = [
        {
           key          = "service_description"
           mode         = "is"
           type         = "fixed"
           values       = []
           include_null = true
         }
       ]
    }
}
`, rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.incl_null",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"include_null": knownvalue.Bool(true),
								"values":       knownvalue.ListExact([]knownvalue.Check{}),
							}),
						}),
					),
				},
			},
			// Drift check.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "incl_null" {
    name = "%s-incl-null"
    description = "test include_null with empty values"
    rule = {
       formula = "A"
       components = [
        {
           key          = "service_description"
           mode         = "is"
           type         = "fixed"
           values       = []
           include_null = true
         }
       ]
    }
}
`, rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAllocation_GroupImportActionPreservation tests that importing a group
// allocation correctly handles the "action" field which is not returned by the API.
// After import, re-applying the same config should produce no drift.
func TestAccAllocation_GroupImportActionPreservation(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create a group allocation.
			{
				Config: testAccAllocationGroup(rName),
			},
			// Step 2: Import and verify state matches (rules ignored due to action).
			{
				ResourceName:      "doit_allocation.group",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"update_time", // Computed field
					"rules",       // API doesn't return the 'action' field
				},
			},
			// Step 3: After import, re-apply original config. This verifies
			// that the imported state doesn't cause unnecessary updates.
			{
				Config: testAccAllocationGroup(rName),
			},
			// Step 4: Drift check after re-apply.
			{
				Config: testAccAllocationGroup(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
