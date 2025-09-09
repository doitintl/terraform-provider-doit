package doit_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"golang.org/x/exp/rand"
)

func TestAccAllocationGroup(t *testing.T) {
	n1 := rand.Int()
	n2 := rand.Int()
	n3 := rand.Int()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationGroup(n1, n2, n3),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation_group.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			{
				Config: testAccAllocationGroupUpdated(n1, n2, n3),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_allocation_group.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_allocation_group.this",
						tfjsonpath.New("unallocated_costs"),
						knownvalue.StringExact("unallocated"),
					),
					statecheck.ExpectKnownValue(
						"doit_allocation_group.this",
						tfjsonpath.New("rules").AtSliceIndex(1).AtMapKey("components"),
						knownvalue.Null(),
					),
					statecheck.ExpectKnownValue(
						"doit_allocation_group.this",
						tfjsonpath.New("rules").AtSliceIndex(2).AtMapKey("components").AtSliceIndex(0),
						knownvalue.MapPartial(map[string]knownvalue.Check{
							"values": knownvalue.ListExact(
								[]knownvalue.Check{knownvalue.StringExact("FR")},
							),
						}),
					),
				},
			},
		},
	})
}

func testAccAllocationGroup(n1, n2, n3 int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "name" {
  name        = "test-%d"
  rule = {
    formula = "A AND B"
    components = [{
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
    }]
  }
}

resource "doit_allocation" "name2" {
  name        = "test-%d"
  rule = {
    formula = "A AND B"
    components = [{
      key    = "country"
      mode   = "is"
      type   = "fixed"
      values = ["DE"]
      },
      {
        key    = "project_id"
        mode   = "is"
        type   = "fixed"
        values = ["test-k8s-project-468707"]
    }]
  }
}

resource "doit_allocation_group" "this" {
  name = "test-%d"
  rules = [
   {
      action     = "select"
      id         = resource.doit_allocation.name.id
   },
   {
      action     = "select"
      id         = resource.doit_allocation.name2.id
      // support optional provided components case for 'select'
      components = resource.doit_allocation.name2.rule.components
   },
   {
	  action     = "create"
      name       = "test-rule-%d"
      description = "Terraform test rule"
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
		  values = ["test-k8s-project-468707"]
		},
      ],
      formula = "A OR B"
	},
  ]
}

`, n1, n2, n3, n3)
}

func testAccAllocationGroupUpdated(n1, n2, n3 int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "name" {
  name        = "test-%d"
  rule = {
    formula = "A AND B"
    components = [{
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
    }]
  }
}

resource "doit_allocation" "name2" {
  name        = "test-%d"
  rule = {
    formula = "A AND B"
    components = [{
      key    = "country"
      mode   = "is"
      type   = "fixed"
      values = ["DE"]
      },
      {
        key    = "project_id"
        mode   = "is"
        type   = "fixed"
        values = ["test-k8s-project-468707"]
    }]
  }
}

resource "doit_allocation_group" "this" {
  name = "test-%d"
  unallocated_costs = "unallocated"
  description = "Terraform test"
  rules = [
	{
	  action     = "select"
	  id         = resource.doit_allocation.name.id
 	},
	{
	  action     = "select"
	  id         = resource.doit_allocation.name2.id
	},
	{
	  action     = "create"
      name       = "test-rule-%d"
      description = "Terraform test rule"
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
		  values = ["test-k8s-project-468707"]
		},
      ],
      formula = "A OR B"
	},
  ]
}

`, n1, n2, n3, n3)
}
