package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccDimensionDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Use doit_dimensions (list) to discover the first dimension,
				// then feed its type and id into doit_dimension (singular).
				Config: testAccDimensionDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The singular data source should have the same id and type
					// as the first item from the list data source
					resource.TestCheckResourceAttrPair(
						"data.doit_dimension.test", "id",
						"data.doit_dimensions.first", "dimensions.0.id",
					),
					resource.TestCheckResourceAttrPair(
						"data.doit_dimension.test", "type",
						"data.doit_dimensions.first", "dimensions.0.type",
					),
					resource.TestCheckResourceAttrSet("data.doit_dimension.test", "label"),
					resource.TestCheckResourceAttrSet("data.doit_dimension.test", "values.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccDimensionDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccDimensionDataSourceConfig() string {
	return `
# Discover the first available dimension from the list
data "doit_dimensions" "first" {
  max_results = "1"
}

# Look up that dimension's details (including values) using the singular DS
data "doit_dimension" "test" {
  type = data.doit_dimensions.first.dimensions[0].type
  id   = data.doit_dimensions.first.dimensions[0].id
}
`
}

// TestAccDimensionDataSource_AllocationTypes verifies that the replacement and
// legacy dimension types both resolve, and that state keeps the configured
// spelling even though the API reports "allocation_rule"/"allocation" for both
// (#443). The API pairs each type with its own id, so they change together.
func TestAccDimensionDataSource_AllocationTypes(t *testing.T) {
	cases := map[string][2]string{
		"allocation_rule": {"allocation_rule", "allocation_rule"},
		"attribution":     {"attribution", "attribution"},
	}
	if groupID := os.Getenv("TEST_ATTRIBUTION_GROUP"); groupID != "" {
		cases["allocation"] = [2]string{"allocation", groupID}
		cases["attribution_group"] = [2]string{"attribution_group", groupID}
	}

	for name, tc := range cases {
		typ, id := tc[0], tc[1]
		t.Run(name, func(t *testing.T) {
			config := fmt.Sprintf(`
data "doit_dimension" "test" {
  type = %q
  id   = %q
}
`, typ, id)
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: config,
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("data.doit_dimension.test", "type", typ),
							resource.TestCheckResourceAttr("data.doit_dimension.test", "id", id),
							resource.TestCheckResourceAttrSet("data.doit_dimension.test", "label"),
						),
					},
					{
						Config: config,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}
