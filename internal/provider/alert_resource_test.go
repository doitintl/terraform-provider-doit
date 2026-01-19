package provider_test

import (
	"fmt"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAlert(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Alert Create
			{
				Config: testAccAlert(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_alert.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-alert-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("time_interval"),
						knownvalue.StringExact("month")),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("value"),
						knownvalue.Float64Exact(1000)),
				},
			},
			// Test Alert Update (In-place)
			{
				Config: testAccAlertUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_alert.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-alert-updated-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("value"),
						knownvalue.Float64Exact(2000)),
				},
			},
		},
	})
}

func TestAccAlert_Import(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlert(n),
			},
			{
				ResourceName:      "doit_alert.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAlert_WithScopes(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithScopes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("fixed"),
								"id":     knownvalue.StringExact("cloud_provider"),
								"mode":   knownvalue.StringExact("is"),
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("amazon-web-services")}),
							}),
						}),
					),
				},
			},
		},
	})
}

func testAccAlert(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
  recipients = ["%s"]
}
`, i, testUser())
}

func testAccAlertUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-updated-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 2000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
  recipients = ["%s"]
}
`, i, testUser())
}

func testAccAlertWithScopes(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-scopes-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "fixed"
        id     = "cloud_provider"
        mode   = "is"
        values = ["amazon-web-services"]
      }
    ]
  }
}
`, i)
}
