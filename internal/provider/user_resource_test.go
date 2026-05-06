package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

func testAccInviteEmail(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_INVITE_EMAIL")
	if v == "" {
		t.Skip("TEST_INVITE_EMAIL must be set for this test")
	}
	return v
}

// deleteTestUser is a helper that removes a user by email via the API.
// Used by test cleanup to ensure idempotent test runs.
func deleteTestUser(t *testing.T, email string) {
	t.Helper()
	client := getAPIClient(t)

	params := &models.ListUsersParams{Email: new(openapi_types.Email(email))}

	listResp, err := client.ListUsersWithResponse(context.Background(), params)
	if err != nil {
		t.Logf("Warning: could not list users for cleanup: %v", err)
		return
	}
	if listResp.StatusCode() != 200 || listResp.JSON200 == nil {
		t.Logf("Warning: ListUsers returned %d during cleanup", listResp.StatusCode())
		return
	}
	if listResp.JSON200.Users == nil || len(*listResp.JSON200.Users) == 0 {
		return // user doesn't exist, nothing to clean up
	}

	users := *listResp.JSON200.Users
	if users[0].Id == nil {
		t.Logf("Warning: user found but has no internal ID during cleanup")
		return
	}

	deleteResp, err := client.DeleteUserWithResponse(context.Background(), *users[0].Id)
	if err != nil {
		t.Logf("Warning: could not delete user during cleanup: %v", err)
		return
	}
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		t.Logf("Warning: delete user returned %d during cleanup", deleteResp.StatusCode())
	}
}

// TestAccUser_Basic tests inviting a user with only the email field and
// verifies no drift on re-apply.
func TestAccUser_Basic(t *testing.T) {
	email := testAccInviteEmail(t)

	// Clean up any leftover user from a previous failed run.
	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with email only.
			{
				Config: testAccUserBasic(email),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(email)),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("email"),
						knownvalue.StringExact(email)),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("status"),
						knownvalue.StringExact("invited")),
				},
			},
			// Step 2: Drift check — re-apply same config, expect no changes.
			{
				Config: testAccUserBasic(email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccUser_AllFields tests inviting a user with all configurable fields
// (including phone/language which trigger the auto-PATCH flow).
func TestAccUser_AllFields(t *testing.T) {
	email := testAccInviteEmail(t)

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with all fields.
			{
				Config: testAccUserAllFields(email),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(email)),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("email"),
						knownvalue.StringExact(email)),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("first_name"),
						knownvalue.StringExact("Test")),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("last_name"),
						knownvalue.StringExact("User")),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("job_title"),
						knownvalue.StringExact("Software / Ops Engineer")),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("language"),
						knownvalue.StringExact("en")),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("status"),
						knownvalue.StringExact("invited")),
				},
			},
			// Step 2: Drift check.
			{
				Config: testAccUserAllFields(email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccUser_Update tests modifying updatable fields after initial invite.
func TestAccUser_Update(t *testing.T) {
	email := testAccInviteEmail(t)

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with basic fields.
			{
				Config: testAccUserBasicWithName(email, "Initial", "User"),
			},
			// Step 2: Update name and add language.
			{
				Config: testAccUserUpdated(email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_user.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("first_name"),
						knownvalue.StringExact("Updated")),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("last_name"),
						knownvalue.StringExact("Name")),
					statecheck.ExpectKnownValue(
						"doit_user.test",
						tfjsonpath.New("language"),
						knownvalue.StringExact("ja")),
				},
			},
			// Step 3: Drift check.
			{
				Config: testAccUserUpdated(email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccUser_Import tests importing an existing user by email.
func TestAccUser_Import(t *testing.T) {
	email := testAccInviteEmail(t)

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the user.
			{
				Config: testAccUserBasic(email),
			},
			// Step 2: Import by email.
			{
				ResourceName:      "doit_user.test",
				ImportState:       true,
				ImportStateId:     email,
				ImportStateVerify: true,
			},
			// Step 3: Drift check — re-apply config after import.
			{
				Config: testAccUserBasic(email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccUser_Disappears tests that Terraform detects an externally deleted user.
func TestAccUser_Disappears(t *testing.T) {
	email := testAccInviteEmail(t)

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the user.
			{
				Config: testAccUserBasic(email),
			},
			// Step 2: Delete via API, verify Terraform detects drift.
			{
				PreConfig: func() {
					deleteTestUser(t, email)
				},
				Config:             testAccUserBasic(email),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// --- Config helpers ---

func testAccUserBasic(email string) string {
	return fmt.Sprintf(`
resource "doit_user" "test" {
  email = %q
}
`, email)
}

func testAccUserBasicWithName(email, firstName, lastName string) string {
	return fmt.Sprintf(`
resource "doit_user" "test" {
  email      = %q
  first_name = %q
  last_name  = %q
}
`, email, firstName, lastName)
}

func testAccUserAllFields(email string) string {
	return fmt.Sprintf(`
resource "doit_user" "test" {
  email           = %q
  first_name      = "Test"
  last_name       = "User"
  job_title       = "Software / Ops Engineer"
  language        = "en"
  phone           = "+44"
  phone_extension = "12345"
}
`, email)
}

func testAccUserUpdated(email string) string {
	return fmt.Sprintf(`
resource "doit_user" "test" {
  email      = %q
  first_name = "Updated"
  last_name  = "Name"
  language   = "ja"
}
`, email)
}
