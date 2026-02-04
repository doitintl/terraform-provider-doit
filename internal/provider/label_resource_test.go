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

func TestAccLabel(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Label Create
			{
				Config: testAccLabel(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_label.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-label-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("color"),
						knownvalue.StringExact("blue")),
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("type"),
						knownvalue.StringExact("custom")),
				},
			},
			// Test Label Update (change name and color)
			{
				Config: testAccLabelUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_label.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-label-updated-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("color"),
						knownvalue.StringExact("mint")),
				},
			},
		},
	})
}

func TestAccLabel_Import(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabel(n),
			},
			{
				ResourceName:      "doit_label.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLabel(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "this" {
  name  = "test-label-%d"
  color = "blue"
}
`, i)
}

func testAccLabelUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "this" {
  name  = "test-label-updated-%d"
  color = "mint"
}
`, i)
}

// TestAccLabel_Lavender tests that the API accepts the "lavender" color.
func TestAccLabel_Lavender(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_label" "lavender_test" {
  name  = "test-lavender-%d"
  color = "lavender"
}
`, n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label.lavender_test",
						tfjsonpath.New("color"),
						knownvalue.StringExact("lavender")),
				},
			},
		},
	})
}

// TestAccLabel_InvalidColor tests that invalid colors are rejected.
func TestAccLabel_InvalidColor(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_label" "invalid_test" {
  name  = "test-invalid-color-%d"
  color = "invalid_color"
}
`, n),
				ExpectError: regexp.MustCompile(`(?i)value must be one of:`),
			},
		},
	})
}
