package provider_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// The API has no notion of "inline" group members: a rule sent with action="create" becomes an
// ordinary single allocation, identical to one created through a separate doit_allocation and
// referenced with action="select". The provider therefore only knows which members it created
// from the action kept in state, and must delete exactly those when the group goes away or a
// create rule is removed. These tests record member ids from state and query the API directly,
// because CheckDestroy only sees resources that are still tracked in state.

// recordGroupRuleIDs stores rules[*].id of the given group, keyed by rule name, into ids.
func recordGroupRuleIDs(resourceName string, ids map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		count, err := strconv.Atoi(rs.Primary.Attributes["rules.#"])
		if err != nil {
			return fmt.Errorf("parsing rules.#: %w", err)
		}
		for i := range count {
			name := rs.Primary.Attributes[fmt.Sprintf("rules.%d.name", i)]
			id := rs.Primary.Attributes[fmt.Sprintf("rules.%d.id", i)]
			if id == "" {
				return fmt.Errorf("rules.%d (%s) has no id in state", i, name)
			}
			ids[name] = id
		}
		return nil
	}
}

// recordResourceID stores the id of a standalone doit_allocation under key.
func recordResourceID(resourceName, key string, ids map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		ids[key] = rs.Primary.ID
		return nil
	}
}

// allocationStatus returns the HTTP status of GET /allocations/{id}.
func allocationStatus(t *testing.T, id string) (int, error) {
	t.Helper()
	resp, err := getAPIClient(t).GetAllocationWithResponse(t.Context(), id)
	if err != nil {
		return 0, fmt.Errorf("error checking allocation %s: %w", id, err)
	}
	return resp.StatusCode(), nil
}

// checkRecordedAllocations asserts that each named recorded allocation id either still exists
// (wantExists) or returns 404.
func checkRecordedAllocations(t *testing.T, ids map[string]string, wantExists bool, names ...string) error {
	t.Helper()
	for _, name := range names {
		id, ok := ids[name]
		if !ok {
			return fmt.Errorf("no recorded id for rule %q", name)
		}
		status, err := allocationStatus(t, id)
		if err != nil {
			return err
		}
		if wantExists && status != 200 {
			return fmt.Errorf("allocation %q (%s) should still exist, got status %d", name, id, status)
		}
		if !wantExists && status != 404 {
			return fmt.Errorf("allocation %q (%s) should have been deleted, got status %d", name, id, status)
		}
	}
	return nil
}

func testAccCheckRecordedAllocationsExist(t *testing.T, ids map[string]string, names ...string) resource.TestCheckFunc {
	return func(_ *terraform.State) error { return checkRecordedAllocations(t, ids, true, names...) }
}

func testAccCheckRecordedAllocationsGone(t *testing.T, ids map[string]string, names ...string) resource.TestCheckFunc {
	return func(_ *terraform.State) error { return checkRecordedAllocations(t, ids, false, names...) }
}

// testAccCheckGroupMembersDestroy runs the standard destroy check and additionally asserts the
// named recorded members are gone (or still present when wantExists is set).
func testAccCheckGroupMembersDestroy(t *testing.T, ids map[string]string, wantExists bool, names ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := testAccCheckAllocationDestroy(t)(s); err != nil {
			return err
		}
		return checkRecordedAllocations(t, ids, wantExists, names...)
	}
}

// Destroying a group deletes every allocation it created with action="create".
func TestAccAllocation_Group_CreateRulesDeletedOnDestroy(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	ids := map[string]string{}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy: testAccCheckGroupMembersDestroy(t, ids, false,
			rName+"-jp", rName+"-us"),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupCreateRules(rName, "jp", "us"),
				Check: resource.ComposeAggregateTestCheckFunc(
					recordGroupRuleIDs("doit_allocation.group", ids),
					testAccCheckRecordedAllocationsExist(t, ids, rName+"-jp", rName+"-us"),
				),
			},
			{
				Config: testAccAllocationGroupCreateRules(rName, "jp", "us"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// A created rule that references another managed allocation must not block that allocation's
// destroy (the API answers 409 "allocation single is in use" while the created rule exists).
func TestAccAllocation_Group_CreateRuleReferencingSingleDestroys(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	ids := map[string]string{}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckGroupMembersDestroy(t, ids, false, rName+"-ref"),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupCreateRuleReferencingSingle(rName),
				Check:  recordGroupRuleIDs("doit_allocation.group", ids),
			},
		},
	})
}

// Destroying a group never deletes allocations it merely selected, and in a mixed group only the
// created member is removed.
func TestAccAllocation_Group_SelectMembersSurviveGroupDestroy(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	ids := map[string]string{}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckGroupMembersDestroy(t, ids, false, rName+"-created", rName+"-selected"),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupMixedCreateSelect(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					recordGroupRuleIDs("doit_allocation.group", ids),
					recordResourceID("doit_allocation.selected", rName+"-selected", ids),
				),
			},
			// Remove only the group from config; its created member goes, the selected single stays.
			{
				Config: testAccAllocationGroupMixedCreateSelect(rName, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRecordedAllocationsGone(t, ids, rName+"-created"),
					testAccCheckRecordedAllocationsExist(t, ids, rName+"-selected"),
				),
			},
		},
	})
}

// Removing a created rule from a group deletes the allocation it created; renaming a rule keeps
// it, and the remaining rule is untouched.
func TestAccAllocation_GroupUpdate_RemovedCreateRuleDeleted(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	ids := map[string]string{}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckGroupMembersDestroy(t, ids, false, rName+"-jp", rName+"-us"),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupCreateRules(rName, "jp", "us"),
				Check:  recordGroupRuleIDs("doit_allocation.group", ids),
			},
			{
				Config: testAccAllocationGroupCreateRules(rName, "jp"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRecordedAllocationsGone(t, ids, rName+"-us"),
					testAccCheckRecordedAllocationsExist(t, ids, rName+"-jp"),
				),
			},
			{
				Config: testAccAllocationGroupCreateRules(rName, "jp"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// Rules written with action="update" are the same inline definitions (the provider sends "update"
// for create rules that already have an id), so they are deleted with the group and on removal.
func TestAccAllocation_Group_UpdateActionRulesDeleted(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	ids := map[string]string{}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckGroupMembersDestroy(t, ids, false, rName+"-jp", rName+"-us"),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupRulesWithAction(rName, "create", "jp", "us"),
				Check:  recordGroupRuleIDs("doit_allocation.group", ids),
			},
			// Switch the verb to "update" for the same rules; ids and members are unchanged.
			{
				Config: testAccAllocationGroupRulesWithAction(rName, "update", "jp", "us"),
				Check:  testAccCheckRecordedAllocationsExist(t, ids, rName+"-jp", rName+"-us"),
			},
			// Dropping an "update" rule deletes its allocation.
			{
				Config: testAccAllocationGroupRulesWithAction(rName, "update", "jp"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRecordedAllocationsGone(t, ids, rName+"-us"),
					testAccCheckRecordedAllocationsExist(t, ids, rName+"-jp"),
				),
			},
			// Destroy (end of test) deletes the remaining one.
		},
	})
}

func TestAccAllocation_GroupUpdate_RenamedCreateRuleKept(t *testing.T) {
	rName := acctest.RandomWithPrefix(testAllocPrefix)
	ids := map[string]string{}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckGroupMembersDestroy(t, ids, false, rName+"-jp"),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroupCreateRules(rName, "jp"),
				Check:  recordGroupRuleIDs("doit_allocation.group", ids),
			},
			{
				Config: testAccAllocationGroupCreateRulesRenamed(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRecordedAllocationsExist(t, ids, rName+"-jp"),
					resource.TestCheckResourceAttrWith("doit_allocation.group", "rules.0.id", func(v string) error {
						if v != ids[rName+"-jp"] {
							return fmt.Errorf("renamed rule id changed from %s to %s", ids[rName+"-jp"], v)
						}
						return nil
					}),
				),
			},
		},
	})
}

func testAccGroupCountryRule(rName, key, country, action string) string {
	return fmt.Sprintf(`
    {
      action  = "%s"
      name    = "%s-%s"
      formula = "A"
      components = [
        {
          key    = "country"
          mode   = "is"
          type   = "fixed"
          values = ["%s"]
        }
      ]
    }`, action, rName, key, country)
}

var testAccGroupCountries = map[string]string{"jp": "JP", "us": "US"}

// testAccAllocationGroupCreateRules builds a group whose rules are all action="create", one per key
// ("jp", "us"), named <rName>-<key>.
func testAccAllocationGroupCreateRules(rName string, keys ...string) string {
	return testAccAllocationGroupRulesWithAction(rName, "create", keys...)
}

// testAccAllocationGroupRulesWithAction is testAccAllocationGroupCreateRules with a chosen action.
func testAccAllocationGroupRulesWithAction(rName, action string, keys ...string) string {
	rules := ""
	for i, k := range keys {
		if i > 0 {
			rules += ","
		}
		rules += testAccGroupCountryRule(rName, k, testAccGroupCountries[k], action)
	}
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
  name              = "%s-group"
  description       = "group of created rules"
  unallocated_costs = "%s-other"
  rules = [%s
  ]
}
`, rName, rName, rules)
}

func testAccAllocationGroupCreateRulesRenamed(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
  name              = "%s-group"
  description       = "group of created rules"
  unallocated_costs = "%s-other"
  rules = [
    {
      action  = "create"
      name    = "%s-jp-renamed"
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
`, rName, rName, rName)
}

func testAccAllocationGroupCreateRuleReferencingSingle(rName string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "single" {
  name        = "%s-single"
  description = "single referenced by a created rule"
  rule = {
    formula = "A"
    components = [
      {
        key    = "country"
        mode   = "is"
        type   = "fixed"
        values = ["DE"]
      }
    ]
  }
}

resource "doit_allocation" "group" {
  name              = "%s-group"
  description       = "created rule referencing a managed single"
  unallocated_costs = "%s-other"
  rules = [
    {
      action  = "create"
      name    = "%s-ref"
      formula = "A"
      components = [
        {
          key    = "allocation_rule"
          mode   = "is"
          type   = "allocation_rule"
          values = [doit_allocation.single.id]
        }
      ]
    }
  ]
}
`, rName, rName, rName, rName)
}

// testAccAllocationGroupMixedCreateSelect configures a managed single plus, when withGroup is set,
// a group that selects the single and creates one more rule.
func testAccAllocationGroupMixedCreateSelect(rName string, withGroup bool) string {
	cfg := fmt.Sprintf(`
resource "doit_allocation" "selected" {
  name        = "%s-selected"
  description = "single selected by a group"
  rule = {
    formula = "A"
    components = [
      {
        key    = "country"
        mode   = "is"
        type   = "fixed"
        values = ["FR"]
      }
    ]
  }
}
`, rName)
	if !withGroup {
		return cfg
	}
	return cfg + fmt.Sprintf(`
resource "doit_allocation" "group" {
  name              = "%s-group"
  description       = "one created and one selected member"
  unallocated_costs = "%s-other"
  rules = [
    {
      action = "select"
      id     = doit_allocation.selected.id
    },
    {
      action  = "create"
      name    = "%s-created"
      formula = "A"
      components = [
        {
          key    = "country"
          mode   = "is"
          type   = "fixed"
          values = ["IT"]
        }
      ]
    }
  ]
}
`, rName, rName, rName)
}
