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

// TestAccAllocation_InverseConflict tests that setting both inverse=true and
// inverse_selection=true on the same component is rejected at plan time.
func TestAccAllocation_InverseConflict(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationInverseConflict(rName),
				ExpectError: regexp.MustCompile(`Conflicting Inverse Attributes`),
			},
		},
	})
}

func testAccAllocationInverseConflict(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inv_conflict" {
    name        = "%s-inv-conflict"
    description = "test allocation with conflicting inverse attributes"
    rule = {
       formula = "A"
       components = [
        {
           key               = "country"
           mode              = "is"
           type              = "fixed"
           values            = ["JP"]
           inverse           = true
           inverse_selection = true
         }
       ]
    }
}
`, rName)
}

// TestAccAllocation_InverseConflict_Rules verifies that the conflict validator
// also fires on the rules[] (group allocation) path, not just rule.
func TestAccAllocation_InverseConflict_Rules(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationInverseConflictRules(rName),
				ExpectError: regexp.MustCompile(`Conflicting Inverse Attributes`),
			},
		},
	})
}

func testAccAllocationInverseConflictRules(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inv_conflict_rules" {
    name              = "%s-inv-conflict-rules"
    description       = "test allocation group with conflicting inverse attributes"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-conflict-rule"
            formula = "A"
            components = [
                {
                    key               = "country"
                    mode              = "is"
                    type              = "fixed"
                    values            = ["JP"]
                    inverse           = true
                    inverse_selection = true
                }
            ]
        }
    ]
}
`, rName, rName, rName)
}

// TestAccAllocation_InverseRulesPath tests that inverse=true works correctly
// through the rules[] (group allocation) path, not just the rule path.
func TestAccAllocation_InverseRulesPath(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with inverse=true via rules[]
			{
				Config: testAccAllocationInverseRulesPath(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_allocation.inv_rules", "id"),
					resource.TestCheckResourceAttr("doit_allocation.inv_rules", "rules.0.components.0.inverse", "true"),
				),
			},
			// Step 2: Drift check — plan should be empty
			{
				Config: testAccAllocationInverseRulesPath(rName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationInverseRulesPath(rName string, inverse bool) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inv_rules" {
    name              = "%s-inv-rules"
    description       = "test allocation group with inverse via rules path"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-inv-rule"
            formula = "A"
            components = [
                {
                    key     = "country"
                    mode    = "is"
                    type    = "fixed"
                    values  = ["JP"]
                    inverse = %t
                }
            ]
        }
    ]
}
`, rName, rName, rName, inverse)
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

// TestAccAllocation_InverseLifecycle comprehensively tests the "inverse" attribute
// through its full lifecycle: create with true, drift check, toggle to false, toggle
// back to true, and import + drift check. This verifies that:
//   - The provider correctly sends inverse=true to the API without interference
//     from the deprecated inverse_selection field.
//   - The Read path detects real API values (no masking from prior state).
//   - No "Provider produced inconsistent result" errors occur in any transition.
func TestAccAllocation_InverseLifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with inverse=true.
			{
				Config: testAccAllocationInverseLifecycle(rName, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.inv_lifecycle",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"inverse":           knownvalue.Bool(true),
								"inverse_selection": knownvalue.Bool(false),
							}),
						}),
					),
				},
			},
			// Step 2: Drift check — re-apply same config, expect no changes.
			{
				Config: testAccAllocationInverseLifecycle(rName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Update to inverse=false.
			{
				Config: testAccAllocationInverseLifecycle(rName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.inv_lifecycle",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.inv_lifecycle",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"inverse":           knownvalue.Bool(false),
								"inverse_selection": knownvalue.Bool(false),
							}),
						}),
					),
				},
			},
			// Step 4: Drift check after update to false.
			{
				Config: testAccAllocationInverseLifecycle(rName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 5: Update back to inverse=true.
			{
				Config: testAccAllocationInverseLifecycle(rName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.inv_lifecycle",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.inv_lifecycle",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"inverse":           knownvalue.Bool(true),
								"inverse_selection": knownvalue.Bool(false),
							}),
						}),
					),
				},
			},
			// Step 6: Drift check after update to true.
			{
				Config: testAccAllocationInverseLifecycle(rName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 7: Import + verify state.
			{
				ResourceName:      "doit_allocation.inv_lifecycle",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"update_time", // Computed field that changes on each modification
				},
			},
			// Step 8: Drift check after import.
			{
				Config: testAccAllocationInverseLifecycle(rName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationInverseLifecycle(rName string, inverse bool) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inv_lifecycle" {
    name        = "%s-inv-lifecycle"
    description = "Test allocation inverse lifecycle"
    rule = {
       formula = "A"
       components = [
        {
           key     = "country"
           mode    = "is"
           type    = "fixed"
           values  = ["JP"]
           inverse = %t
         }
       ]
    }
}
`, rName, inverse)
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
// attribute (and back) does not produce "inconsistent result" errors.
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
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.inv_migrate",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"inverse_selection": knownvalue.Bool(true),
							}),
						}),
					),
				},
			},
			// Step 2: Drift check for legacy config.
			{
				Config: testAccAllocationInverseSelectionLegacy(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Migrate to inverse = true (remove inverse_selection).
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
			// Step 4: Drift check after migration to inverse.
			{
				Config: testAccAllocationInverseMigrated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 5: Migrate back to inverse_selection = true (remove inverse).
			{
				Config: testAccAllocationInverseSelectionLegacy(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.inv_migrate",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"inverse_selection": knownvalue.Bool(true),
							}),
						}),
					),
				},
			},
			// Step 6: Drift check after migrating back.
			{
				Config: testAccAllocationInverseSelectionLegacy(rName),
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

// TestAccAllocation_InverseWithMultipleComponents tests an allocation with
// multiple components using different combinations of inverse and inverse_selection.
// This verifies that the write path correctly handles each component independently
// and that the read path doesn't cross-contaminate values between components.
func TestAccAllocation_InverseWithMultipleComponents(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with 3 components:
			//   A: no inverse (default false)
			//   B: inverse = true
			//   C: inverse_selection = true (deprecated)
			{
				Config: testAccAllocationInverseMultiComponent(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.inv_multi",
						tfjsonpath.New("rule").AtMapKey("components"),
						knownvalue.ListExact([]knownvalue.Check{
							// A: default inverse=false
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":               knownvalue.StringExact("country"),
								"inverse":           knownvalue.Bool(false),
								"inverse_selection": knownvalue.Bool(false),
							}),
							// B: inverse=true
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":               knownvalue.StringExact("service_description"),
								"inverse":           knownvalue.Bool(true),
								"inverse_selection": knownvalue.Bool(false),
							}),
							// C: inverse_selection=true (deprecated)
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":               knownvalue.StringExact("project_id"),
								"inverse_selection": knownvalue.Bool(true),
							}),
						}),
					),
				},
			},
			// Step 2: Drift check.
			{
				Config: testAccAllocationInverseMultiComponent(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationInverseMultiComponent(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "inv_multi" {
    name        = "%s-inv-multi"
    description = "Test allocation with mixed inverse/inverse_selection components"
    rule = {
       formula = "A AND B AND C"
       components = [
        {
           key    = "country"
           mode   = "is"
           type   = "fixed"
           values = ["JP"]
        },
        {
           key     = "service_description"
           mode    = "is"
           type    = "fixed"
           values  = ["AmazonCloudWatch"]
           inverse = true
        },
        {
           key               = "project_id"
           mode              = "is"
           type              = "fixed"
           values            = ["%s"]
           inverse_selection = true
        }
       ]
    }
}
`, rName, testProject())
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

// TestAccAllocation_CanonicalServiceName tests that a single allocation with a
// long canonical service_description value creates successfully and produces no
// drift on re-apply.
//
// Asserts:
//   - Step 1: Create with canonical name succeeds
//   - Step 2: Re-apply produces empty plan (no drift)
func TestAccAllocation_CanonicalServiceName(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with the canonical service name.
			{
				Config: testAccAllocationCanonicalServiceName(rName),
			},
			// Step 2: Verify no drift on re-apply.
			{
				Config: testAccAllocationCanonicalServiceName(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAllocation_SuffixedValueRejected verifies that the API rejects filter
// values with known suffixes (e.g. "(EKS)") with a 400 error.
//
// Historical context: The API previously silently stripped known suffixes. As of
// April 2026, it now returns a 400 error instructing the user to use the
// canonical name. This test ensures we detect if the API behavior changes again.
//
// Asserts:
//   - Step 1: Create with suffixed value fails with expected error message
func TestAccAllocation_SuffixedValueRejected(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationSuffixedServiceName(rName),
				ExpectError: regexp.MustCompile(`invalid filter value`),
			},
		},
	})
}

func testAccAllocationCanonicalServiceName(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "norm" {
    name        = "%s-value-norm"
    description = "test allocation with canonical service name"
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

func testAccAllocationSuffixedServiceName(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "norm" {
    name        = "%s-value-norm"
    description = "test allocation with suffixed service name"
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

// TestAccAllocation_GroupUpdate_ExplicitActionUpdate tests updating a group allocation
// where the config explicitly sets action = "update" and passes the rule id from state.
func TestAccAllocation_GroupUpdate_ExplicitActionUpdate(t *testing.T) {
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
			},
			// Step 2: Update with explicit action = "update" and id reference.
			{
				Config: testAccAllocationGroupExplicitUpdate(rName),
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
			// Step 3: Drift check.
			{
				Config: testAccAllocationGroupExplicitUpdate(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupExplicitUpdate(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "test allocation group updated explicitly"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "update"
            name    = "%s-jp-rule"
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
            action  = "update"
            name    = "%s-us-rule"
            formula = "A AND B"
            components = [
                {
                    key    = "country"
                    mode   = "is"
                    type   = "fixed"
                    values = ["FR"]
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

// TestAccAllocation_GroupUpdate_AddAndRemoveRules tests adding a rule to an existing group
// allocation, and then removing a rule from it.
func TestAccAllocation_GroupUpdate_AddAndRemoveRules(t *testing.T) {
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
			},
			// Step 2: Add a 3rd rule.
			{
				Config: testAccAllocationGroupThreeRules(rName),
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
			// Step 3: Remove the second rule (scale back to 2 rules).
			{
				Config: testAccAllocationGroup(rName),
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
			// Step 4: Drift check.
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

func testAccAllocationGroupThreeRules(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "test allocation group three rules"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-jp-rule"
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
            action  = "create"
            name    = "%s-us-rule"
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
        },
        {
            action  = "create"
            name    = "%s-uk-rule"
            formula = "A AND B"
            components = [
                {
                    key    = "country"
                    mode   = "is"
                    type   = "fixed"
                    values = ["GB"]
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
`, rName, rName, rName, testProject(), rName, testProject(), rName, testProject())
}

// TestAccAllocation_GroupUpdate_MixedActions tests updating a group allocation
// containing both in-line rules (action = "create") and selected rules (action = "select").
func TestAccAllocation_GroupUpdate_MixedActions(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create single allocation + group allocation referencing it via select action.
			{
				Config: testAccAllocationGroupMixed(rName, "initial"),
			},
			// Step 2: Update group allocation description.
			{
				Config: testAccAllocationGroupMixed(rName, "updated"),
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
			// Step 3: Drift check.
			{
				Config: testAccAllocationGroupMixed(rName, "updated"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupMixed(rName, descSuffix string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "single" {
    name        = "%s-standalone"
    description = "standalone single allocation for select rule"
    rule = {
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
}

resource "doit_allocation" "group" {
    name              = "%s-group"
    description       = "test allocation group mixed %s"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-inline-rule"
            formula = "A"
            components = [
                {
                    key    = "country"
                    mode   = "is"
                    type   = "fixed"
                    values = ["JP"]
                }
            ]
        },
        {
            action = "select"
            id     = doit_allocation.single.id
        }
    ]
}
`, rName, rName, descSuffix, rName, rName)
}

// TestAccAllocation_GroupUpdate_RenameRule tests renaming an in-line rule on an existing group allocation.
func TestAccAllocation_GroupUpdate_RenameRule(t *testing.T) {
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
			},
			// Step 2: Rename rule 1 from jp-rule to japan-rule.
			{
				Config: testAccAllocationGroupRenamedRule(rName),
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
			// Step 3: Drift check.
			{
				Config: testAccAllocationGroupRenamedRule(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupRenamedRule(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "test allocation group renamed rule"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-japan-rule"
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
            action  = "create"
            name    = "%s-us-rule"
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

// TestAccAllocation_GroupUpdate_DeleteFirstRule tests deleting the first rule from a group allocation.
func TestAccAllocation_GroupUpdate_DeleteFirstRule(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create a group allocation with JP and US rules.
			{
				Config: testAccAllocationGroup(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "2"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-jp-rule"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.1.name", rName+"-us-rule"),
				),
			},
			// Step 2: Remove the first rule (JP), leaving only US rule.
			{
				Config: testAccAllocationGroupOnlyUS(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.group",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "1"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-us-rule"),
				),
			},
			// Step 3: Drift check.
			{
				Config: testAccAllocationGroupOnlyUS(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupOnlyUS(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "test allocation group only US rule"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-us-rule"
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
`, rName, rName, rName, testProject())
}

// TestAccAllocation_GroupUpdate_ReorderRules tests reordering rules in a group allocation.
func TestAccAllocation_GroupUpdate_ReorderRules(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create a group allocation with JP at index 0, US at index 1.
			{
				Config: testAccAllocationGroup(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "2"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-jp-rule"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.1.name", rName+"-us-rule"),
				),
			},
			// Step 2: Swap order: US at index 0, JP at index 1.
			{
				Config: testAccAllocationGroupReordered(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.group",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "2"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-us-rule"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.1.name", rName+"-jp-rule"),
				),
			},
			// Step 3: Drift check.
			{
				Config: testAccAllocationGroupReordered(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupReordered(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "test allocation group reordered"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-us-rule"
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
        },
        {
            action  = "create"
            name    = "%s-jp-rule"
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
    ]
}
`, rName, rName, rName, testProject(), rName, testProject())
}

// TestAccAllocation_GroupUpdate_SelectToCreate tests transitioning a rule from action="select" to action="create".
func TestAccAllocation_GroupUpdate_SelectToCreate(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create standalone allocation and group allocation referencing it via action="select".
			{
				Config: testAccAllocationGroupSelect(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group_select", "rules.#", "1"),
					resource.TestCheckResourceAttr("doit_allocation.group_select", "rules.0.action", "select"),
				),
			},
			// Step 2: Update rule to action="create", creating an in-line rule instead.
			{
				Config: testAccAllocationGroupSelectConvertedToCreate(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.group_select",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group_select", "rules.#", "1"),
					resource.TestCheckResourceAttr("doit_allocation.group_select", "rules.0.action", "create"),
					resource.TestCheckResourceAttr("doit_allocation.group_select", "rules.0.name", rName+"-inline-rule"),
				),
			},
			// Step 3: Drift check.
			{
				Config: testAccAllocationGroupSelectConvertedToCreate(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupSelectConvertedToCreate(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "this" {
    name        = "%s-source"
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
    name        = "%s-group-select"
    description = "test allocation group select"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "%s-inline-rule"
            formula = "A AND B"
            components = [
                {
                    key    = "country"
                    mode   = "is"
                    type   = "fixed"
                    values = ["FR"]
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
`, rName, rName, rName, rName, testProject())
}

// TestAccAllocation_GroupUpdate_CreateToUpdateWithoutId tests updating a rule by setting action="update"
// without specifying an id in HCL, while also removing an earlier rule.
func TestAccAllocation_GroupUpdate_CreateToUpdateWithoutId(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create group allocation with JP (index 0) and US (index 1) with action="create".
			{
				Config: testAccAllocationGroup(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "2"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-jp-rule"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.1.name", rName+"-us-rule"),
				),
			},
			// Step 2: Remove JP, set action="update" on US without id in HCL.
			{
				Config: testAccAllocationGroupOnlyUSExplicitUpdate(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.group",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "1"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-us-rule"),
				),
			},
			// Step 3: Drift check.
			{
				Config: testAccAllocationGroupOnlyUSExplicitUpdate(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationGroupOnlyUSExplicitUpdate(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "test allocation group only US rule explicit update"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "update"
            name    = "%s-us-rule"
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
`, rName, rName, rName, testProject())
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

// TestAccAllocation_FolderId verifies that allocations can be created inside a
// folder, the folder_id is persisted in state, the allocation can be moved to
// root, and that re-applying produces no drift.
func TestAccAllocation_FolderId(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create allocation inside a folder
			{
				Config: testAccAllocationInFolder(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.folder_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"doit_allocation.folder_test", "folder_id",
						"doit_folder.alloc_test", "id"),
				),
			},
			// Step 2: Drift check — re-apply same config, expect no changes
			{
				Config: testAccAllocationInFolder(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Move allocation to root (folder_id = "root")
			{
				Config: testAccAllocationInRoot(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation.folder_test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation.folder_test",
						tfjsonpath.New("folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 4: Drift check after move
			{
				Config: testAccAllocationInRoot(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationInFolder(rName string) string {
	return fmt.Sprintf(`
resource "doit_folder" "alloc_test" {
    name = "%s-alloc-folder"
}

resource "doit_allocation" "folder_test" {
    name        = "%s-in-folder"
    description = "Folder test allocation"
    folder_id   = doit_folder.alloc_test.id
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
`, rName, rName, testProject())
}

func testAccAllocationInRoot(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "folder_test" {
    name        = "%s-in-folder"
    description = "Folder test allocation"
    folder_id   = "root"
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
`, rName, testProject())
}

// TestAccAllocation_Computed_Required_Fields tests that using a computed value
// (like an ID from another resource) for a required field (like name or description)
// works correctly and does not trigger any Unknown pointer bugs during Create.
func TestAccAllocation_Computed_Required_Fields(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "dependency" {
    name = "%s-dep"
    description = "Dependency allocation"
    rule = {
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
}

resource "doit_allocation" "dependent" {
    // These required fields will be Unknown at plan time
    name = doit_allocation.dependency.id
    description = "Dependent on ${doit_allocation.dependency.id}"
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
					statecheck.ExpectKnownValue("doit_allocation.dependent", tfjsonpath.New("name"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestAccAllocation_GroupUpdate_UnknownRuleElements verifies that ModifyPlan handles unknown rule elements
// (e.g., when a rule attribute is derived from an unknown computed resource attribute) without failing the plan.
// Step 1 creates the group; Step 2 updates the description so ModifyPlan runs with prior state and
// unknown nested component values (from the computed doit_allocation.dep.id dependency).
func TestAccAllocation_GroupUpdate_UnknownRuleElements(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with a computed dependency in a component value.
			{
				Config: testAccAllocationGroupUnknownRuleElements(rName, "initial description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			// Step 2: Update description so ModifyPlan runs with prior state.
			// The computed dependency (doit_allocation.dep.id) keeps components
			// unknown during planning, exercising the allowUnknown=true path.
			{
				Config: testAccAllocationGroupUnknownRuleElements(rName, "updated description"),
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
		},
	})
}

func testAccAllocationGroupUnknownRuleElements(rName, description string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "dep" {
    name        = "%s-dep"
    description = "Dependency allocation"
    rule = {
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
}

resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "%s"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "create"
            name    = "rule-with-computed-component"
            formula = "A AND B"
            components = [
                {
                    key    = "country"
                    mode   = "is"
                    type   = "fixed"
                    values = ["FR", doit_allocation.dep.id]
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
`, rName, rName, description, rName, testProject())
}

// TestAccAllocation_GroupUpdate_DeleteAndRenameRuleWithoutId tests deleting an earlier rule
// while simultaneously renaming a later rule without specifying an explicit id.
// This reproduces Copilot's finding where Pass 2 positional matching incorrectly assigns the deleted rule's ID.
func TestAccAllocation_GroupUpdate_DeleteAndRenameRuleWithoutId(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckAllocationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create group allocation with JP (index 0) and US (index 1).
			{
				Config: testAccAllocationGroup(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "2"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-jp-rule"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.1.name", rName+"-us-rule"),
				),
			},
			// Step 2: Delete JP rule (index 0) and rename US rule to us-rule-renamed without supplying an explicit id.
			{
				Config: fmt.Sprintf(`
resource "doit_allocation" "group" {
    name        = "%s-group"
    description = "Group with 1 renamed rule"
    unallocated_costs = "%s-other"
    rules = [
        {
            action  = "update"
            name    = "%s-us-rule-renamed"
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
`, rName, rName, rName, testProject()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.#", "1"),
					resource.TestCheckResourceAttr("doit_allocation.group", "rules.0.name", rName+"-us-rule-renamed"),
				),
			},
		},
	})
}
