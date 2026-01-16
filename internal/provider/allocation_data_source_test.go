package provider_test

import (
	"fmt"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAllocationDataSource(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationDataSourceConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify data source attributes match resource
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-ds-%d", n))),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("test allocation for data source")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("single")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("rule").AtMapKey("formula"),
						knownvalue.StringExact("A")),
				},
			},
		},
	})
}

func TestAccAllocationDataSource_Group(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationDataSourceGroupConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("group")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("unallocated_costs"),
						knownvalue.StringExact("Other")),
				},
			},
		},
	})
}

func testAccAllocationDataSourceConfig(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "test" {
    name        = "test-ds-%d"
    description = "test allocation for data source"
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

data "doit_allocation" "test" {
    id = doit_allocation.test.id
}
`, i, testProject())
}

func testAccAllocationDataSourceGroupConfig(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "test" {
    name        = "test-ds-group-%d"
    description = "test allocation group for data source"
    unallocated_costs = "Other"
    rules = [
        {
            action  = "create"
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
    ]
}

data "doit_allocation" "test" {
    id = doit_allocation.test.id
}
`, i, testProject())
}
