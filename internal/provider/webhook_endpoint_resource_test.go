package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccWebhookEndpoint(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookEndpoint(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_webhook_endpoint.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("tf-test-webhook-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.this",
						tfjsonpath.New("url"),
						knownvalue.StringExact("https://hooks.example.com/test")),
				},
			},
			// Update name and url
			{
				Config: testAccWebhookEndpointUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_webhook_endpoint.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("tf-test-webhook-updated-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.this",
						tfjsonpath.New("url"),
						knownvalue.StringExact("https://hooks.example.com/test-updated")),
				},
			},
			// Drift check
			{
				Config: testAccWebhookEndpointUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWebhookEndpoint_Import(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookEndpoint(n),
			},
			{
				ResourceName:      "doit_webhook_endpoint.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Drift check after import
			{
				Config: testAccWebhookEndpoint(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWebhookEndpoint_WithEvents(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookEndpointWithEvents(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.with_events",
						tfjsonpath.New("events"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Drift check
			{
				Config: testAccWebhookEndpointWithEvents(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWebhookEndpoint_WithEmptyEvents(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookEndpointWithEmptyEvents(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.empty_events",
						tfjsonpath.New("events"),
						knownvalue.ListSizeExact(0)),
				},
			},
			// Drift check
			{
				Config: testAccWebhookEndpointWithEmptyEvents(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWebhookEndpoint_WithOmittedEvents(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookEndpoint(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.this",
						tfjsonpath.New("events"),
						knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Drift check
			{
				Config: testAccWebhookEndpoint(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWebhookEndpoint_WithDescription(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookEndpointWithDescription(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.with_desc",
						tfjsonpath.New("description"),
						knownvalue.StringExact(fmt.Sprintf("Test webhook description %d", n))),
				},
			},
			// Drift check
			{
				Config: testAccWebhookEndpointWithDescription(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccWebhookEndpoint_ClearDescription verifies the Category A clearing lifecycle
// for the description field: set → drift check → clear → drift check.
func TestAccWebhookEndpoint_ClearDescription(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with description SET
			{
				Config: testAccWebhookEndpointWithDescription(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.with_desc",
						tfjsonpath.New("description"),
						knownvalue.StringExact(fmt.Sprintf("Test webhook description %d", n))),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccWebhookEndpointWithDescription(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear description (omit from config) — should trigger Update
			{
				Config: testAccWebhookEndpoint(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_webhook_endpoint.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Step 4: Drift check — confirms cleared value is stable
			{
				Config: testAccWebhookEndpoint(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccWebhookEndpoint_ClearEvents verifies the Category A clearing lifecycle
// for the events list: set → drift check → clear → drift check.
func TestAccWebhookEndpoint_ClearEvents(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with events SET
			{
				Config: testAccWebhookEndpointWithEvents(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_webhook_endpoint.with_events",
						tfjsonpath.New("events"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccWebhookEndpointWithEvents(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear events (omit from config) — should trigger Update
			{
				Config: testAccWebhookEndpointNoEvents(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_webhook_endpoint.with_events",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Step 4: Drift check — confirms cleared value is stable
			{
				Config: testAccWebhookEndpointNoEvents(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccWebhookEndpoint_Disappears(t *testing.T) {
	n := acctest.RandInt()
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookEndpoint(n),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("doit_webhook_endpoint.this", "id", func(value string) error {
						if value == "" {
							return fmt.Errorf("resource ID is empty")
						}
						resourceId = value
						return nil
					}),
				),
			},
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteWebhookEndpointWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete webhook endpoint via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204, or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccWebhookEndpoint(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccWebhookEndpoint(i int) string {
	return fmt.Sprintf(`
resource "doit_webhook_endpoint" "this" {
  name = "tf-test-webhook-%d"
  url  = "https://hooks.example.com/test"
}
`, i)
}

func testAccWebhookEndpointUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_webhook_endpoint" "this" {
  name = "tf-test-webhook-updated-%d"
  url  = "https://hooks.example.com/test-updated"
}
`, i)
}

func testAccWebhookEndpointWithEvents(i int) string {
	return fmt.Sprintf(`
resource "doit_webhook_endpoint" "with_events" {
  name = "tf-test-webhook-events-%d"
  url  = "https://hooks.example.com/test-events"
  events = [
    "alert.triggered",
    "alert.resolved",
  ]
}
`, i)
}

func testAccWebhookEndpointNoEvents(i int) string {
	return fmt.Sprintf(`
resource "doit_webhook_endpoint" "with_events" {
  name = "tf-test-webhook-events-%d"
  url  = "https://hooks.example.com/test-events"
}
`, i)
}

func testAccWebhookEndpointWithEmptyEvents(i int) string {
	return fmt.Sprintf(`
resource "doit_webhook_endpoint" "empty_events" {
  name   = "tf-test-webhook-empty-events-%d"
  url    = "https://hooks.example.com/test-empty"
  events = []
}
`, i)
}

func testAccWebhookEndpointWithDescription(i int) string {
	return fmt.Sprintf(`
resource "doit_webhook_endpoint" "with_desc" {
  name        = "tf-test-webhook-%d"
  url         = "https://hooks.example.com/test"
  description = "Test webhook description %d"
}
`, i, i)
}
