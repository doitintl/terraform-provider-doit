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

// TestAccResourceSharing_Basic creates a report and sets sharing permissions (owner + viewer),
// then verifies no drift on re-apply.
func TestAccResourceSharing_Basic(t *testing.T) {
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
				Config: testAccResourceSharingBasic(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_resource_sharing.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("resource_type"),
						knownvalue.StringExact("reports")),
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("id"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("name"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Drift detection - re-apply same config, expect no changes
			{
				Config: testAccResourceSharingBasic(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccResourceSharing_Update tests updating sharing permissions (add user, change roles).
func TestAccResourceSharing_Update(t *testing.T) {
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
				Config: testAccResourceSharingBasic(n),
			},
			// Step 2: Update - promote viewer to editor
			{
				Config: testAccResourceSharingUpdated(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_resource_sharing.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 3: Drift detection
			{
				Config: testAccResourceSharingUpdated(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccResourceSharing_ChangeOwner tests transferring ownership to another user
// and then transferring it back so the CI user can delete the report at cleanup.
func TestAccResourceSharing_ChangeOwner(t *testing.T) {
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
				Config: testAccResourceSharingBasic(n),
			},
			// Step 2: Transfer ownership to TEST_USER_2
			{
				Config: testAccResourceSharingChangedOwner(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 3: Drift detection
			{
				Config: testAccResourceSharingChangedOwner(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 4: Transfer ownership back to CI user so destroy succeeds
			{
				Config: testAccResourceSharingBasic(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccResourceSharing_PublicAccess tests setting, changing, and clearing public access.
// The custom plan modifier (useNullForUnknownWhenConfigNull) allows clearing public
// by explicitly setting public = null in config.
func TestAccResourceSharing_PublicAccess(t *testing.T) {
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
				Config: testAccResourceSharingPublic(n, "viewer"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("public"),
						knownvalue.StringExact("viewer")),
				},
			},
			// Step 2: Drift detection
			{
				Config: testAccResourceSharingPublic(n, "viewer"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Change public access from viewer to editor
			{
				Config: testAccResourceSharingPublic(n, "editor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("public"),
						knownvalue.StringExact("editor")),
				},
			},
			// Step 4: Drift detection
			{
				Config: testAccResourceSharingPublic(n, "editor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 5: Remove public access with explicit null
			{
				Config: testAccResourceSharingPublicNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_resource_sharing.this",
						tfjsonpath.New("public"),
						knownvalue.Null()),
				},
			},
			// Step 6: Drift detection after null
			{
				Config: testAccResourceSharingPublicNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccResourceSharing_Import tests importing existing sharing permissions.
func TestAccResourceSharing_Import(t *testing.T) {
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
				Config: testAccResourceSharingBasic(n),
			},
			// Step 2: Import using composite ID
			{
				ResourceName: "doit_resource_sharing.this",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["doit_resource_sharing.this"]
					if !ok {
						return "", fmt.Errorf("resource not found: doit_resource_sharing.this")
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

// TestAccResourceSharing_OwnerValidator tests the exactly-one-owner validator.
func TestAccResourceSharing_OwnerValidator(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test: No owners
			{
				Config:      testAccResourceSharingNoOwner(n),
				ExpectError: regexp.MustCompile(`Missing Owner`),
			},
			// Test: Multiple owners
			{
				Config:      testAccResourceSharingMultipleOwners(n),
				ExpectError: regexp.MustCompile(`Multiple Owners`),
			},
		},
	})
}

// TestAccResourceSharing_Disappears verifies Terraform handles externally deleted
// target resources. When the report is deleted, the sharing GET returns 404.
func TestAccResourceSharing_Disappears(t *testing.T) {
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
				Config: testAccResourceSharingBasic(n),
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
				Config:             testAccResourceSharingBasic(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// --- HCL Config Helpers ---

func testAccResourceSharingReport(i int) string {
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

func testAccResourceSharingBasic(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_resource_sharing" "this" {
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
`, testAccResourceSharingReport(i), testUser(), testUser2())
}

func testAccResourceSharingUpdated(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_resource_sharing" "this" {
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
`, testAccResourceSharingReport(i), testUser(), testUser2())
}

func testAccResourceSharingChangedOwner(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_resource_sharing" "this" {
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
`, testAccResourceSharingReport(i), testUser2(), testUser())
}

func testAccResourceSharingPublic(i int, publicLevel string) string {
	return fmt.Sprintf(`
%s

resource "doit_resource_sharing" "this" {
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
`, testAccResourceSharingReport(i), testUser(), testUser2(), publicLevel)
}

func testAccResourceSharingPublicNull(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_resource_sharing" "this" {
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
`, testAccResourceSharingReport(i), testUser(), testUser2())
}

func testAccResourceSharingNoOwner(i int) string {
	return fmt.Sprintf(`
resource "doit_resource_sharing" "this" {
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

func testAccResourceSharingMultipleOwners(i int) string {
	return fmt.Sprintf(`
resource "doit_resource_sharing" "this" {
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
