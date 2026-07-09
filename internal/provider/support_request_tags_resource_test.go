package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSupportRequestTags_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // shares ticket with Import test
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestTagsConfig([]string{"tf-test-billing", "tf-test-review"}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_support_request_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("tf-test-billing"),
							knownvalue.StringExact("tf-test-review"),
						}),
					),
					statecheck.ExpectKnownValue(
						"doit_support_request_tags.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
				},
			},
			// Drift verification
			{
				Config: testAccSupportRequestTagsConfig([]string{"tf-test-billing", "tf-test-review"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Update: remove one tag, add another
			{
				Config: testAccSupportRequestTagsConfig([]string{"tf-test-billing", "tf-test-urgent"}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_support_request_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("tf-test-billing"),
							knownvalue.StringExact("tf-test-urgent"),
						}),
					),
				},
			},
			// Drift verification after update
			{
				Config: testAccSupportRequestTagsConfig([]string{"tf-test-billing", "tf-test-urgent"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Clear: an empty set is valid and removes all managed tags.
			{
				Config: testAccSupportRequestTagsConfig([]string{}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_support_request_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.SetSizeExact(0),
					),
				},
			},
			// Drift verification after clearing
			{
				Config: testAccSupportRequestTagsConfig([]string{}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccSupportRequestTags_Normalization verifies that when the API normalizes
// submitted tags (trim + lowercase), the resource preserves the user's original
// representation in state and produces no drift on a subsequent plan.
func TestAccSupportRequestTags_Normalization(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // shares ticket with Basic/Import tests
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestTagsConfig([]string{"Tf-Test-Mixed", "tf-test-lower"}),
				ConfigStateChecks: []statecheck.StateCheck{
					// The user's exact representation is preserved even though the
					// API stores/returns the trim+lowercased form.
					statecheck.ExpectKnownValue(
						"doit_support_request_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("Tf-Test-Mixed"),
							knownvalue.StringExact("tf-test-lower"),
						}),
					),
				},
			},
			// Drift verification: no spurious diff from API normalization.
			{
				Config: testAccSupportRequestTagsConfig([]string{"Tf-Test-Mixed", "tf-test-lower"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccSupportRequestTags_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // shares ticket with Basic test
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestTagsConfig([]string{"tf-test-import"}),
			},
			{
				ResourceName:      "doit_support_request_tags.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Re-apply after import to reconcile tags, then verify no drift
			{
				Config: testAccSupportRequestTagsConfig([]string{"tf-test-import"}),
			},
			{
				Config: testAccSupportRequestTagsConfig([]string{"tf-test-import"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccSupportRequestTagsConfig(tags []string) string {
	tagsList := ""
	for i, t := range tags {
		if i > 0 {
			tagsList += ", "
		}
		tagsList += fmt.Sprintf("%q", t)
	}

	return fmt.Sprintf(`
data "doit_support_requests" "all" {
  max_results = 1
}

resource "doit_support_request_tags" "test" {
  ticket_id = data.doit_support_requests.all.tickets[0].id
  tags      = [%s]
}
`, tagsList)
}
