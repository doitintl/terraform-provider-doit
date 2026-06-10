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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func testUser2() string {
	return os.Getenv("TEST_USER_2")
}

// TestAccSharing_Basic creates a report and sets sharing permissions (owner + viewer),
// then verifies no drift on re-apply.
func TestAccSharing_Basic(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create report + sharing with owner + viewer
			{
				Config: testAccSharingBasic(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_sharing.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("resource_type"),
						knownvalue.StringExact("reports")),
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("id"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("name"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Drift detection - re-apply same config, expect no changes
			{
				Config: testAccSharingBasic(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccSharing_Update tests updating sharing permissions (add user, change roles).
func TestAccSharing_Update(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with owner + viewer
			{
				Config: testAccSharingBasic(n),
			},
			// Step 2: Update - promote viewer to editor
			{
				Config: testAccSharingUpdated(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_sharing.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 3: Drift detection
			{
				Config: testAccSharingUpdated(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccSharing_ChangeOwner tests transferring ownership to another user
// and then transferring it back so the CI user can delete the report at cleanup.
func TestAccSharing_ChangeOwner(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with CI user as owner
			{
				Config: testAccSharingBasic(n),
			},
			// Step 2: Transfer ownership to TEST_USER_2
			{
				Config: testAccSharingChangedOwner(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 3: Drift detection
			{
				Config: testAccSharingChangedOwner(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 4: Transfer ownership back to CI user so destroy succeeds
			{
				Config: testAccSharingBasic(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccSharing_PublicAccess tests setting, changing, and clearing public access.
// The custom plan modifier (useEmptyForUnknownWhenConfigNull) allows clearing public
// by explicitly setting public = null in config.
func TestAccSharing_PublicAccess(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with public = viewer
			{
				Config: testAccSharingPublic(n, "viewer"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("public"),
						knownvalue.StringExact("viewer")),
				},
			},
			// Step 2: Drift detection
			{
				Config: testAccSharingPublic(n, "viewer"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Change public access from viewer to editor
			{
				Config: testAccSharingPublic(n, "editor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("public"),
						knownvalue.StringExact("editor")),
				},
			},
			// Step 4: Drift detection
			{
				Config: testAccSharingPublic(n, "editor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 5: Remove public access with explicit null
			{
				Config: testAccSharingPublicNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.this",
						tfjsonpath.New("public"),
						knownvalue.Null()),
				},
			},
			// Step 6: Drift detection after null
			{
				Config: testAccSharingPublicNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccSharing_Import tests importing existing sharing permissions.
func TestAccSharing_Import(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccSharingBasic(n),
			},
			// Step 2: Import using composite ID
			{
				ResourceName: "doit_sharing.this",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["doit_sharing.this"]
					if !ok {
						return "", fmt.Errorf("resource not found: doit_sharing.this")
					}
					resType := rs.Primary.Attributes["resource_type"]
					resID := rs.Primary.Attributes["resource_id"]
					return fmt.Sprintf("%s/%s", resType, resID), nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}

// TestAccSharing_OwnerValidator tests the exactly-one-owner validator.
func TestAccSharing_OwnerValidator(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test: No owners
			{
				Config:      testAccSharingNoOwner(n),
				ExpectError: regexp.MustCompile(`Missing Owner`),
			},
			// Test: Multiple owners
			{
				Config:      testAccSharingMultipleOwners(n),
				ExpectError: regexp.MustCompile(`Multiple Owners`),
			},
			// Test: public set on allocations (not allowed)
			{
				Config:      testAccSharingAllocationWithPublic(n),
				ExpectError: regexp.MustCompile(`Public Access Not Supported for Allocations`),
			},
		},
	})
}

// TestAccSharing_Disappears verifies Terraform handles externally deleted
// target resources. When the report is deleted, the sharing GET returns 404.
func TestAccSharing_Disappears(t *testing.T) {
	n := acctest.RandInt()
	var reportID string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resources
			{
				Config: testAccSharingBasic(n),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("doit_report.sharing_target", "id", func(value string) error {
						if value == "" {
							return fmt.Errorf("resource ID is empty")
						}
						reportID = value
						return nil
					}),
				),
			},
			// Step 2: Delete the report via API, then verify Terraform detects the drift
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteReportWithResponse(context.Background(), reportID)
					if err != nil {
						t.Fatalf("Failed to delete report via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Unexpected status code when deleting report: %d, body: %s",
							resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccSharingBasic(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// --- HCL Config Helpers ---

func testAccSharingReport(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "sharing_target" {
  name = "test-sharing-target-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    aggregation    = "total"
    time_interval  = "month"
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}
`, i)
}

func testAccSharingBasic(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_sharing" "this" {
  resource_type = "reports"
  resource_id   = doit_report.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "viewer"
    }
  ]
}
`, testAccSharingReport(i), testUser(), testUser2())
}

func testAccSharingUpdated(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_sharing" "this" {
  resource_type = "reports"
  resource_id   = doit_report.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "editor"
    }
  ]
}
`, testAccSharingReport(i), testUser(), testUser2())
}

func testAccSharingChangedOwner(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_sharing" "this" {
  resource_type = "reports"
  resource_id   = doit_report.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "editor"
    }
  ]
}
`, testAccSharingReport(i), testUser2(), testUser())
}

func testAccSharingPublic(i int, publicLevel string) string {
	return fmt.Sprintf(`
%s

resource "doit_sharing" "this" {
  resource_type = "reports"
  resource_id   = doit_report.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "viewer"
    }
  ]

  public = "%s"
}
`, testAccSharingReport(i), testUser(), testUser2(), publicLevel)
}

func testAccSharingPublicNull(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_sharing" "this" {
  resource_type = "reports"
  resource_id   = doit_report.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "viewer"
    }
  ]

  public = null
}
`, testAccSharingReport(i), testUser(), testUser2())
}

func testAccSharingNoOwner(i int) string {
	return fmt.Sprintf(`
resource "doit_sharing" "this" {
  resource_type = "reports"
  resource_id   = "fake-id-%d"

  permissions = [
    {
      user = "someone@example.com"
      role = "viewer"
    }
  ]
}
`, i)
}

func testAccSharingMultipleOwners(i int) string {
	return fmt.Sprintf(`
resource "doit_sharing" "this" {
  resource_type = "reports"
  resource_id   = "fake-id-%d"

  permissions = [
    {
      user = "owner1@example.com"
      role = "owner"
    },
    {
      user = "owner2@example.com"
      role = "owner"
    }
  ]
}
`, i)
}

func testAccSharingAllocationWithPublic(i int) string {
	return fmt.Sprintf(`
resource "doit_sharing" "this" {
  resource_type = "allocations"
  resource_id   = "fake-id-%d"
  public        = "viewer"

  permissions = [
    {
      user = "owner@example.com"
      role = "owner"
    }
  ]
}
`, i)
}

// --- Tests for other resource types (budgets, alerts, allocations) ---

// TestAccSharing_Budget tests sharing permissions on a budget resource.
func TestAccSharing_Budget(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
			if testAttribution() == "" {
				t.Skip("TEST_ATTRIBUTION must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSharingBudget(n),
				// The sharing API syncs permissions to budget collaborators server-side,
				// causing expected drift on the doit_budget resource.
				ExpectNonEmptyPlan: true,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.budget",
						tfjsonpath.New("resource_type"),
						knownvalue.StringExact("budgets")),
					statecheck.ExpectKnownValue(
						"doit_sharing.budget",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 2: Align the budget's collaborators with the sharing permissions.
			// When both resources declare the same users, drift disappears entirely.
			{
				Config: testAccSharingBudgetAligned(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccSharing_Alert tests sharing permissions on an alert resource.
func TestAccSharing_Alert(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSharingAlert(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.alert",
						tfjsonpath.New("resource_type"),
						knownvalue.StringExact("alerts")),
					statecheck.ExpectKnownValue(
						"doit_sharing.alert",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Drift check
			{
				Config: testAccSharingAlert(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccSharing_Allocation tests sharing permissions on an allocation resource.
func TestAccSharing_Allocation(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if testUser2() == "" {
				t.Skip("TEST_USER_2 must be set for this test")
			}
			if testProject() == "" {
				t.Skip("TEST_PROJECT must be set for this test")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSharingAllocation(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_sharing.allocation",
						tfjsonpath.New("resource_type"),
						knownvalue.StringExact("allocations")),
					statecheck.ExpectKnownValue(
						"doit_sharing.allocation",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Drift check
			{
				Config: testAccSharingAllocation(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// --- Config generators for other resource types ---

func testAccSharingBudget(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "sharing_target" {
  name          = "sharing-test-budget-%d"
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
  type         = "recurring"
  start_period = local.start_period
}

resource "doit_sharing" "budget" {
  resource_type = "budgets"
  resource_id   = doit_budget.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "viewer"
    }
  ]
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser(), testUser2())
}

// testAccSharingBudgetAligned returns a config where the budget's
// collaborators are aligned with the sharing resource's permissions. This
// eliminates cross-resource drift that occurs when the sharing API syncs
// permissions to the budget's collaborators server-side.
func testAccSharingBudgetAligned(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "sharing_target" {
  name          = "sharing-test-budget-%d"
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
    {
      "email" : "%s",
      "role" : "viewer"
    },
  ]
  type         = "recurring"
  start_period = local.start_period
}

resource "doit_sharing" "budget" {
  resource_type = "budgets"
  resource_id   = doit_budget.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "viewer"
    }
  ]
}
`, budgetStartPeriod(), i, testAttribution(), testUser(), testUser2(), testUser(), testUser2())
}
func testAccSharingAlert(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "sharing_target" {
  name = "sharing-test-alert-%d"
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
}

resource "doit_sharing" "alert" {
  resource_type = "alerts"
  resource_id   = doit_alert.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "viewer"
    }
  ]
}
`, i, testUser(), testUser2())
}

func testAccSharingAllocation(i int) string {
	return fmt.Sprintf(`
resource "doit_allocation" "sharing_target" {
  name        = "sharing-test-alloc-%d"
  description = "allocation for sharing test"
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

resource "doit_sharing" "allocation" {
  resource_type = "allocations"
  resource_id   = doit_allocation.sharing_target.id

  permissions = [
    {
      user = "%s"
      role = "owner"
    },
    {
      user = "%s"
      role = "viewer"
    }
  ]
}
`, i, testProject(), testUser(), testUser2())
}
