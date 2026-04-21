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
			// Drift detection: re-apply same config, expect no changes.
			{
				Config: testAccBudget(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
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

	// Verify omitted scope booleans resolve correctly from API response.
	// These booleans are Optional+Computed and arrive as Unknown when the user
	// omits them. The overlay resolves them from apiResp.Scopes[i].
	scopeBoolChecks := []statecheck.StateCheck{
		statecheck.ExpectKnownValue(
			"doit_budget.this",
			tfjsonpath.New("scopes").AtSliceIndex(0).AtMapKey("inverse"),
			knownvalue.Bool(false)),
		statecheck.ExpectKnownValue(
			"doit_budget.this",
			tfjsonpath.New("scopes").AtSliceIndex(0).AtMapKey("include_null"),
			knownvalue.Bool(false)),
		statecheck.ExpectKnownValue(
			"doit_budget.this",
			tfjsonpath.New("scopes").AtSliceIndex(0).AtMapKey("case_insensitive"),
			knownvalue.Bool(false)),
	}

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
				ConfigStateChecks: append([]statecheck.StateCheck{
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
				}, scopeBoolChecks...),
			},
			// Drift detection: re-apply same config, expect no changes.
			// Verifies overlay and Read path agree on boolean resolution.
			{
				Config: testAccBudgetScopes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: scopeBoolChecks,
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
							knownvalue.Float64Exact(100),
							knownvalue.Float64Exact(100),
							knownvalue.Float64Exact(100),
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

  # Covered: seasonal_amounts (all equal to amount to avoid API recomputing amount)
  seasonal_amounts = [100, 100, 100, 100]

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

  # Covered: seasonal_amounts (all equal to amount to avoid API recomputing amount)
  seasonal_amounts = [100, 100, 100, 100]

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

// TestAccBudget_ScopesAliasTypes tests that using the "allocation_rule" alias type
// (for "attribution") in budget scopes round-trips correctly without causing drift.
// The API returns the canonical "attribution" name, but the normalizer preserves
// the user's "allocation_rule" choice in state.
func TestAccBudget_ScopesAliasTypes(t *testing.T) {
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
				Config: testAccBudgetScopesAliasTypes(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify alias type and id are preserved in state
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type": knownvalue.StringExact("allocation_rule"),
								"id":   knownvalue.StringExact("allocation_rule"),
								"mode": knownvalue.StringExact("is"),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply - critical test for alias normalization
			{
				Config: testAccBudgetScopesAliasTypes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetScopesAliasTypes(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-alias-scopes-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type   = "allocation_rule"
      id     = "allocation_rule"
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
`, budgetStartPeriod(), i, testAttribution(), testUser())
}

// TestAccBudget_IncludeNull tests that the include_null property on budget scopes
// round-trips correctly without causing drift.
func TestAccBudget_IncludeNull(t *testing.T) {
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
				Config: testAccBudgetWithIncludeNull(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":         knownvalue.StringExact("fixed"),
								"id":           knownvalue.StringExact("service_description"),
								"mode":         knownvalue.StringExact("is"),
								"include_null": knownvalue.Bool(true),
								"values":       knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("Compute Engine")}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccBudgetWithIncludeNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetWithIncludeNull(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-include-null-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type         = "fixed"
      id           = "service_description"
      mode         = "is"
      include_null = true
      values       = ["Compute Engine"]
    }
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

// TestAccBudget_CaseInsensitive tests that the case_insensitive property on budget scopes
// round-trips correctly without causing drift. Uses mode="contains" with a lowercase
// value to exercise the case-insensitive matching path.
func TestAccBudget_CaseInsensitive(t *testing.T) {
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
				Config: testAccBudgetWithCaseInsensitive(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":             knownvalue.StringExact("fixed"),
								"id":               knownvalue.StringExact("cloud_provider"),
								"mode":             knownvalue.StringExact("contains"),
								"case_insensitive": knownvalue.Bool(true),
								"values":           knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("aws")}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccBudgetWithCaseInsensitive(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetWithCaseInsensitive(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-case-insensitive-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type             = "fixed"
      id               = "cloud_provider"
      mode             = "contains"
      case_insensitive = true
      values           = ["aws"]
    }
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

// TestAccBudget_ScopeIncludeNull tests that a scope with include_null = true
// and a real value round-trips correctly without drift.
// This is the budget equivalent of TestAccReport_IncludeNull.
// It exercises the state-first includeNullVal logic: the API may not reliably
// echo include_null back, so the provider must preserve the user's value.
func TestAccBudget_ScopeIncludeNull(t *testing.T) {
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
				Config: testAccBudgetScopeIncludeNull(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":         knownvalue.StringExact("fixed"),
								"id":           knownvalue.StringExact("service_description"),
								"mode":         knownvalue.StringExact("is"),
								"include_null": knownvalue.Bool(true),
								"values":       knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("Compute Engine")}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccBudgetScopeIncludeNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetScopeIncludeNull(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-scope-include-null-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type         = "fixed"
      id           = "service_description"
      mode         = "is"
      include_null = true
      values       = ["Compute Engine"]
    }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type         = "recurring"
  start_period = local.start_period
}
`, budgetStartPeriod(), i, testUser())
}

// TestAccBudget_ScopeWithNAValue tests that a scope configured with the legacy
// NullFallback sentinel value (e.g. "[Service N/A]") round-trips correctly —
// i.e. after apply the state still contains "[Service N/A]" and no drift is
// reported on a subsequent plan.
//
// This is the budget equivalent of TestAccReport_FilterValuesNAStripped.
//
// budget.go now has the same sentinel-restoration logic that report.go has
// (isNAFallback + populateState restoration), so this test is expected to pass
// and serves as a regression guard for that behavior.
//
// The failure mode (if the restoration logic regresses) is
// "Provider produced inconsistent result after apply": the provider sends
// "[Service N/A]" → the API strips it and sets include_null=true → on read the
// provider gets back include_null=true + values=[] → state becomes values=[],
// which mismatches the configured "[Service N/A]".
// See: https://doitintl.atlassian.net/browse/CMP-38116
func TestAccBudget_ScopeWithNAValue(t *testing.T) {
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
			// Step 1: Create with "[Service N/A]". The API strips this sentinel
			// and returns values=[] + include_null=true. The provider must
			// restore the sentinel in state so it matches the plan exactly.
			{
				Config: testAccBudgetWithNAValue(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type": knownvalue.StringExact("fixed"),
								"id":   knownvalue.StringExact("service_description"),
								"mode": knownvalue.StringExact("is"),
								// The sentinel must be present — not stripped.
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("[Service N/A]")}),
							}),
						}),
					),
				},
			},
			// Step 2: Verify no drift on re-apply — critical regression guard.
			{
				Config: testAccBudgetWithNAValue(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetWithNAValue(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-scope-na-value-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type   = "fixed"
      id     = "service_description"
      mode   = "is"
      values = ["[Service N/A]"]
    }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type         = "recurring"
  start_period = local.start_period
}
`, budgetStartPeriod(), i, testUser())
}

// TestAccBudget_ScopeWithMixedNAValue verifies that a scope containing BOTH a real
// value AND a legacy "[... N/A]" sentinel (e.g. values = ["Compute Engine", "[Service N/A]"])
// round-trips without drift after the sentinel-restoration fix in this PR.
//
// The API strips the sentinel and returns values=["Compute Engine"] + include_null=true.
// mergeSentinelValues (called from budget.go's populateState) detects the discrepancy
// and restores the sentinel in-place, so state continues to match configuration.
// This test is the budget equivalent of TestAccReport_FilterValuesMixedWithNA and
// serves as a regression guard.
// See: https://doitintl.atlassian.net/browse/CMP-38116
func TestAccBudget_ScopeWithMixedNAValue(t *testing.T) {
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
			// Step 1: Create with both a real value and a sentinel.
			// The provider must restore the sentinel after the API strips it,
			// so state contains both values as configured.
			{
				Config: testAccBudgetWithMixedNAValue(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"id":   knownvalue.StringExact("service_description"),
								"mode": knownvalue.StringExact("is"),
								// Both the real value and the sentinel must be present.
								"values": knownvalue.ListExact([]knownvalue.Check{
									knownvalue.StringExact("Compute Engine"),
									knownvalue.StringExact("[Service N/A]"),
								}),
							}),
						}),
					),
				},
			},
			// Step 2: No drift on re-apply.
			{
				Config: testAccBudgetWithMixedNAValue(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetWithMixedNAValue(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-scope-mixed-na-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type   = "fixed"
      id     = "service_description"
      mode   = "is"
      values = ["Compute Engine", "[Service N/A]"]
    }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type         = "recurring"
  start_period = local.start_period
}
`, budgetStartPeriod(), i, testUser())
}

// TestAccBudget_IncludeNullOnlyNoValues tests that a scope with include_null = true
// and NO values is accepted by the API and round-trips without drift.
//
// PR #51575 (fix(analytics): allow include_null and empty values public-api) is
// deployed and the budget API accepts this configuration. This test verifies the
// full round-trip: the provider sends include_null=true with an empty values list,
// the API stores it, and the provider reads it back without drift.
//
// If this test fails with a provider inconsistency error, check that budget.go
// correctly maps a nil/empty API values list to an empty Terraform list (not null)
// when include_null=true is set in the scope.
func TestAccBudget_IncludeNullOnlyNoValues(t *testing.T) {
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
				Config: testAccBudgetIncludeNullOnlyNoValues(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":         knownvalue.StringExact("fixed"),
								"id":           knownvalue.StringExact("service_description"),
								"mode":         knownvalue.StringExact("is"),
								"include_null": knownvalue.Bool(true),
								// values must be empty — no sentinel needed
								"values": knownvalue.ListExact([]knownvalue.Check{}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccBudgetIncludeNullOnlyNoValues(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetIncludeNullOnlyNoValues(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-budget-include-null-only-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type         = "fixed"
      id           = "service_description"
      mode         = "is"
      include_null = true
      values       = []
    }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type         = "recurring"
  start_period = local.start_period
}
`, budgetStartPeriod(), i, testUser())
}

// TestAccBudget_DriftDetection_CustomerPattern tests for drift using the
// customer's exact pattern from ticket 300568: uses metric, growth_per_period,
// recipients_slack_channels=[], and a fixed cloud_provider scope.
// These are the attributes the customer had to add ignore_changes for.
func TestAccBudget_DriftDetection_CustomerPattern(t *testing.T) {
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
				Config: testAccBudgetCustomerPattern(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction("doit_budget.drift_test", plancheck.ResourceActionCreate),
					},
				},
			},
			// Drift detection: re-apply same config, expect no changes.
			// This catches drift from attributes exercised here: amount,
			// recipients_slack_channels, scopes, and alerts[].percentage.
			{
				Config: testAccBudgetCustomerPattern(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetCustomerPattern(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "drift_test" {
  name              = "test-drift-customer-%d"
  currency          = "USD"
  type              = "recurring"
  amount            = 100
  time_interval     = "month"
  growth_per_period = 1
  metric            = "amortized_cost"
  start_period      = local.start_period
  recipients = [
    "%s"
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
  alerts = [
    { "percentage" : 100 },
    { "percentage" : 120 }
  ]
  use_prev_spend = false
  # Customer sets this to empty list — potential drift source
  recipients_slack_channels = []
  scopes = [
    {
      type   = "fixed"
      id     = "cloud_provider"
      mode   = "is"
      values = ["amazon-web-services"]
    }
  ]
}
`, budgetStartPeriod(), i, testUser(), testUser())
}

// TestAccBudget_OmittedOptionalComputed tests that omitting Optional+Computed scalar
// fields does not cause drift. The plan-first overlay must resolve these unknowns
// from the API response (or null) and the Read path must agree.
//
// Critically, this also verifies that omitted list fields (recipients_slack_channels,
// scopes, seasonal_amounts) resolve to empty lists [] — not null — matching the
// Read path (mapBudgetToModel). A null↔[] mismatch would cause state churn.
func TestAccBudget_OmittedOptionalComputed(t *testing.T) {
	n := acctest.RandInt()

	// State checks that verify omitted list fields are empty lists, not null.
	// This directly catches the null↔[] flip between Create/Update and Read.
	omittedListChecks := []statecheck.StateCheck{
		statecheck.ExpectKnownValue(
			"doit_budget.omitted_test",
			tfjsonpath.New("recipients_slack_channels"),
			knownvalue.ListSizeExact(0)),
		statecheck.ExpectKnownValue(
			"doit_budget.omitted_test",
			tfjsonpath.New("scopes"),
			knownvalue.ListSizeExact(0)),
		statecheck.ExpectKnownValue(
			"doit_budget.omitted_test",
			tfjsonpath.New("seasonal_amounts"),
			knownvalue.ListSizeExact(0)),
	}

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
			// Create a minimal budget omitting description, growth_per_period, metric, public,
			// and all optional list fields (recipients_slack_channels, scopes, seasonal_amounts).
			{
				Config: testAccBudgetMinimalOmitted(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction("doit_budget.omitted_test", plancheck.ResourceActionCreate),
					},
				},
				// Verify after create: omitted lists are [] not null
				ConfigStateChecks: omittedListChecks,
			},
			// Drift check: re-apply same config, expect no changes.
			// After Read refreshes state from API, lists must still be [] — proving
			// the overlay and Read path agree on the empty-list representation.
			{
				Config: testAccBudgetMinimalOmitted(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: omittedListChecks,
			},
		},
	})
}

func testAccBudgetMinimalOmitted(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "omitted_test" {
  name          = "test-omitted-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  use_prev_spend = false
  scope         = ["%s"]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
  alerts = [
    { "percentage" : 100 }
  ]
  recipients = ["%s"]
  # description, growth_per_period, metric, public are intentionally omitted
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser())
}

// TestAccBudget_UpdateOmittedField tests updating from a budget with all fields
// set to one where some Optional+Computed fields are omitted, verifying no drift.
func TestAccBudget_UpdateOmittedField(t *testing.T) {
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
			// Step 1: Create with explicit description and growth_per_period
			{
				Config: testAccBudgetWithOptionalFields(n),
			},
			// Step 2: Update to omit description and growth_per_period
			{
				Config: testAccBudgetWithoutOptionalFields(n),
			},
			// Step 3: Drift check
			{
				Config: testAccBudgetWithoutOptionalFields(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetWithOptionalFields(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name              = "test-optional-%d"
  amount            = 100
  currency          = "USD"
  time_interval     = "month"
  type              = "recurring"
  start_period      = local.start_period
  use_prev_spend    = false
  description       = "A test budget with optional fields"
  growth_per_period = 0
  metric            = "cost"
  scope             = ["%s"]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
  alerts = [
    { "percentage" : 80 }
  ]
  recipients = ["%s"]
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser())
}

func testAccBudgetWithoutOptionalFields(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-optional-%d"
  amount        = 200
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  use_prev_spend = false
  scope         = ["%s"]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
  alerts = [
    { "percentage" : 90 }
  ]
  recipients = ["%s"]
  # description, growth_per_period, metric are intentionally omitted
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser())
}

// TestAccBudget_UsePrevSpend tests use_prev_spend = true where the API computes
// the amount from previous spend. Verifies the overlay correctly preserves the
// user's use_prev_spend = true and doesn't drift on the API-computed amount.
func TestAccBudget_UsePrevSpend(t *testing.T) {
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
			// Create with use_prev_spend = true
			{
				Config: testAccBudgetUsePrevSpend(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("use_prev_spend"),
						knownvalue.Bool(true)),
				},
			},
			// Drift check
			{
				Config: testAccBudgetUsePrevSpend(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetUsePrevSpend(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name           = "test-prevspend-%d"
  currency       = "USD"
  time_interval  = "month"
  type           = "recurring"
  start_period   = local.start_period
  use_prev_spend = true
  scope          = ["%s"]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
  alerts = [
    { "percentage" : 100 }
  ]
  recipients = ["%s"]
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser())
}

// TestAccBudget_FixedToRecurring tests converting a fixed budget to recurring.
// The end_period field goes from set→omitted, and the overlay must correctly
// handle this transition without drift.
func TestAccBudget_FixedToRecurring(t *testing.T) {
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
			// Step 1: Create as fixed budget
			{
				Config: testAccBudgetFixedForConversion(n),
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
			// Step 2: Convert to recurring (end_period disappears)
			{
				Config: testAccBudgetRecurringFromFixed(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("type"),
						knownvalue.StringExact("recurring")),
				},
			},
			// Step 3: Drift check
			{
				Config: testAccBudgetRecurringFromFixed(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetFixedForConversion(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name         = "test-fix2rec-%d"
  amount       = 500
  currency     = "USD"
  type         = "fixed"
  start_period = local.start_period
  end_period   = local.start_period + (30 * 24 * 60 * 60 * 1000)
  scope        = ["%s"]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
}
`, budgetStartPeriod(), i, testAttribution(), testUser())
}

func testAccBudgetRecurringFromFixed(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-fix2rec-%d"
  amount        = 500
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scope         = ["%s"]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
  alerts = [
    { "percentage" : 100 }
  ]
  recipients = ["%s"]
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser())
}

// TestAccBudget_PublicField tests the public sharing access level field.
// Verifies the value is preserved by the overlay and survives Read refresh.
func TestAccBudget_PublicField(t *testing.T) {
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
			// Create with public = "viewer"
			{
				Config: testAccBudgetPublicField(n, "viewer"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("public"),
						knownvalue.StringExact("viewer")),
				},
			},
			// Drift check
			{
				Config: testAccBudgetPublicField(n, "viewer"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Update to public = "editor"
			{
				Config: testAccBudgetPublicField(n, "editor"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("public"),
						knownvalue.StringExact("editor")),
				},
			},
			// Drift check after update
			{
				Config: testAccBudgetPublicField(n, "editor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetPublicField(i int, public string) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-public-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  use_prev_spend = false
  public        = "%s"
  scope         = ["%s"]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    }
  ]
  alerts = [
    { "percentage" : 100 }
  ]
  recipients = ["%s"]
}
`, budgetStartPeriod(), i, public, testAttribution(), testUser(), testUser())
}
