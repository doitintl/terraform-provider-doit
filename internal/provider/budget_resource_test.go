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
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccBudget(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Budget (Recurring)
			{
				Config: testAccBudget(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_budget.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-min-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("type"),
						knownvalue.StringExact("recurring")),
				},
			},
			// Test Budget Update (In-place)
			{
				Config: testAccBudgetUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_budget.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("amount"),
						knownvalue.Float64Exact(150)),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-min-updated-%d", n))),
				},
			},
			// Test Fixed Budget
			{
				Config: testAccBudgetFixed(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("type"),
						knownvalue.StringExact("fixed")),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("end_period"),
						knownvalue.NotNull()),
				},
			},
			// Test Full Budget (Recurring)
			{
				Config: testAccBudgetFull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
		},
	})
}

func budgetStartPeriod() string {
	return `
locals {
  start_period = provider::time::rfc3339_parse("2025-10-01T00:00:00Z").unix * 1000
}

output "start_period" {
	value = local.start_period
}
`
}

func testUser() string {
	return os.Getenv("TEST_USER")
}

func testAttribution() string {
	return os.Getenv("TEST_ATTRIBUTION")
}

func testSlackChannel() string {
	return os.Getenv("TEST_SLACK_CHAN")
}

func testCustomerID() string {
	return os.Getenv("TEST_CUSTOMER_ID")
}

func testAccBudget(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-min-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scope         = ["%s"]
  alerts = [
    { percentage = 50 }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i, testAttribution(), testUser())
}

func testAccBudgetUpdate(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-min-updated-%d"
  amount        = 150
  currency      = "EUR"
  time_interval = "month"
  scope         = ["%s"]
  alerts = [
    { percentage = 50 },
    { percentage = 80 }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i, testAttribution(), testUser())
}

func testAccBudgetFixed(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-fixed-%d"
  amount        = 500
  type          = "fixed"
  start_period  = local.start_period
  end_period    = local.start_period + (30 * 24 * 60 * 60 * 1000) # 30 days later
  scope         = ["%s"] # Required by validator
}
`, budgetStartPeriod(), i, testAttribution())
}

func testAccBudgetFull(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name = "test-full-%d"
  description = "test full budget"
  alerts = [
    {
      percentage = 50
    },
    {
      percentage = 85,
    },
    {
      percentage = 100,
    }
  ]
  recipients = [
    "%s"
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  scope = [
    "%s"
  ]
  amount            = 1000
  currency          = "EUR"
  growth_per_period = 5
  time_interval     = "quarter"
  type              = "recurring"
  use_prev_spend    = false
  start_period      = local.start_period
  public            = "viewer"
}
`, budgetStartPeriod(), i, testUser(), testUser(), testAttribution())
}

func TestAccBudget_Import(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudget(n),
			},
			{
				ResourceName:      "doit_budget.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccBudget_Scopes(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetScopes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("fixed"),
								"id":     knownvalue.StringExact("cloud_provider"),
								"mode":   knownvalue.StringExact("is"),
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("amazon-web-services")}),
							}),
						},
						),
					),
				},
			},
		},
	})
}

func TestAccBudget_Conflict(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccBudgetConflict(n),
				ExpectError: regexp.MustCompile("Attributes 'scope' and 'scopes' are mutually exclusive"),
			},
		},
	})
}

func testAccBudgetScopes(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-scopes-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type   = "fixed"
      id     = "cloud_provider"
      mode   = "is"
      values = ["amazon-web-services"]
    }
  ]
  alerts = [
    { percentage = 50 },
    { percentage = 80 },
    { percentage = 100 }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i, testUser())
}

func testAccBudgetConflict(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-conflict-%d"
  amount        = 100
  scope         = ["%s"]
  scopes = [
    {
      type   = "attribution"
      id     = "attribution"
      mode   = "is"
      values = ["%s"]
    }
  ]
  alerts = [
    { percentage = 50 },
    { percentage = 80 },
    { percentage = 100 }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i, testAttribution(), testAttribution(), testUser())
}

func TestAccBudget_Attributes_Coverage(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetAttributesCoverage(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("seasonal_amounts"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.Float64Exact(100),
							knownvalue.Float64Exact(200),
							knownvalue.Float64Exact(300),
							knownvalue.Float64Exact(400),
						})),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("metric"),
						knownvalue.StringExact("cost")),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"inverse": knownvalue.Bool(true),
							}),
						})),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccBudgetAttributesCoverage(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Test metric = "amortized_cost" to verify API round-trip
			{
				Config: testAccBudgetAttributesCoverageAmortizedCost(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("metric"),
						knownvalue.StringExact("amortized_cost")),
				},
			},
			// Verify no drift on re-apply with amortized_cost
			{
				Config: testAccBudgetAttributesCoverageAmortizedCost(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetAttributesCoverage(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name             = "test-attributes-%d"
  currency         = "EUR"
  time_interval    = "quarter"
  type             = "recurring"
  start_period     = local.start_period

  amount           = 100

  # Covered: seasonal_amounts instead of amount
  seasonal_amounts = [100, 200, 300, 400]

  # Covered: metric explicitly
  metric           = "cost"

  # Covered: scopes with inverse
  scopes = [
    {
      type    = "fixed"
      id      = "cloud_provider"
      mode    = "is"
      values  = ["google-cloud-platform"]
      inverse = true
    }
  ]

  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
}
`, budgetStartPeriod(), i, testUser())
}

func testAccBudgetAttributesCoverageAmortizedCost(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name             = "test-attributes-%d"
  currency         = "EUR"
  time_interval    = "quarter"
  type             = "recurring"
  start_period     = local.start_period

  amount           = 100

  # Covered: seasonal_amounts instead of amount
  seasonal_amounts = [100, 200, 300, 400]

  # Covered: metric with amortized_cost value
  metric           = "amortized_cost"

  # Covered: scopes with inverse
  scopes = [
    {
      type    = "fixed"
      id      = "cloud_provider"
      mode    = "is"
      values  = ["google-cloud-platform"]
      inverse = true
    }
  ]

  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
}
`, budgetStartPeriod(), i, testUser())
}

func TestAccBudget_SlackChannel(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetSlackChannel(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("recipients_slack_channels"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"id": knownvalue.StringExact(testSlackChannel()),
							}),
						})),
				},
			},
		},
	})
}

func testAccBudgetSlackChannel(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name             = "test-slack-%d"
  currency         = "EUR"
  time_interval    = "quarter"
  type             = "recurring"
  start_period     = local.start_period

  amount           = 100

  recipients_slack_channels = [
    {
      id          = "%s"
      customer_id = "%s"
    }
  ]

  scope = ["%s"]

  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
}
`, budgetStartPeriod(), i, testSlackChannel(), testCustomerID(), testAttribution(), testUser())
}

// TestAccBudget_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccBudget_Disappears(t *testing.T) {
	n := acctest.RandInt()
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource
			{
				Config: testAccBudget(n),
				Check: resource.ComposeTestCheckFunc(
					// Capture the resource ID for later deletion
					resource.TestCheckResourceAttrWith("doit_budget.this", "id", func(value string) error {
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
					// Delete the resource directly via API
					client := getAPIClient(t)
					resp, err := client.DeleteBudgetWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete budget via API: %v", err)
					}
					// 200 or 204 = success, 404 = already deleted (both are OK)
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Unexpected status code when deleting budget: %d, body: %s",
							resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccBudget(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}

// TestAccBudget_ListAttributes_Collaborators tests all three scenarios for collaborators:
// 1. Explicit collaborator with owner role
// 2. Empty list (validator blocks)
// 3. Omitted (API adds creator as owner).
func TestAccBudget_ListAttributes_Collaborators(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test 1: Empty collaborators = [] blocked by validator
			{
				Config:      testAccBudgetWithEmptyCollaborators(n),
				ExpectError: regexp.MustCompile(`Exactly One Owner Required`),
			},
			// Test 2: Explicit collaborator with owner role
			{
				Config: testAccBudgetWithExplicitCollaborator(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("collaborators"),
						knownvalue.ListSizeExact(1)),
				},
			},
			// Test 3: Omitted collaborators - API adds creator as owner
			{
				Config: testAccBudgetNoCollaborators(n + 1),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("collaborators"),
						knownvalue.ListSizeExact(1)), // API adds creator
				},
			},
		},
	})
}

// TestAccBudget_ListAttributes_AlertsAndRecipients tests all three scenarios for alerts and recipients.
func TestAccBudget_ListAttributes_AlertsAndRecipients(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test 1: Explicit alerts and recipients
			{
				Config: testAccBudgetWithExplicitAlertsAndRecipients(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("alerts"),
						knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("recipients"),
						knownvalue.ListSizeExact(1)),
				},
			},
			// Test 2: Empty alerts=[] and recipients=[]
			// Both are blocked by validators - API doesn't support empty lists for these.
			{
				Config:      testAccBudgetWithEmptyAlertsAndRecipients(n + 1),
				ExpectError: regexp.MustCompile(`Invalid Alerts Configuration|Invalid Recipients Configuration|cannot be empty`),
			},
			// Test 3: Omitted alerts and recipients - API behavior
			// NOTE: When omitted, API provides default alerts, and adds creator as default recipient
			{
				Config: testAccBudgetNoAlertsOrRecipients(n + 2),
				ConfigStateChecks: []statecheck.StateCheck{
					// API provides default alerts when omitted
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("alerts"),
						knownvalue.NotNull()),
					// API adds creator as default recipient
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("recipients"),
						knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

func testAccBudgetWithEmptyCollaborators(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-empty-collab-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scope         = ["%s"]
  collaborators = []
}
`, budgetStartPeriod(), i, testAttribution())
}

func testAccBudgetWithExplicitCollaborator(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-explicit-collab-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scope         = ["%s"]
  collaborators = [
    {
      email = "%s"
      role  = "owner"
    }
  ]
}
`, budgetStartPeriod(), i, testAttribution(), testUser())
}

func testAccBudgetNoCollaborators(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-no-collab-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scope         = ["%s"]
  # collaborators omitted - API adds creator as owner
}
`, budgetStartPeriod(), i, testAttribution())
}

func testAccBudgetWithExplicitAlertsAndRecipients(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-explicit-alerts-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scope         = ["%s"]
  collaborators = [
    {
      email = "%s"
      role  = "owner"
    }
  ]
  alerts = [
    { percentage = 50 },
    { percentage = 100 }
  ]
  recipients = ["%s"]
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser())
}

func testAccBudgetWithEmptyAlertsAndRecipients(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-empty-alerts-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scope         = ["%s"]
  collaborators = [
    {
      email = "%s"
      role  = "owner"
    }
  ]
  alerts     = []
  recipients = []
}
`, budgetStartPeriod(), i, testAttribution(), testUser())
}

func testAccBudgetNoAlertsOrRecipients(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-no-alerts-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scope         = ["%s"]
  collaborators = [
    {
      email = "%s"
      role  = "owner"
    }
  ]
  # alerts and recipients omitted - API computes defaults
}
`, budgetStartPeriod(), i, testAttribution(), testUser())
}
