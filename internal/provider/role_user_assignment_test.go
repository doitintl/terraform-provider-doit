package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// These tests cover roles that are held by invited users. Deleting a role that
// is assigned must succeed and strip the role from its holders; it used to
// fail with 409 "role is assigned to users or groups" once a holder had been
// deleted, because the API's in-use counter was never decremented.
//
// All of them share TEST_INVITE_EMAIL with the user tests, so none can run in
// parallel.

const (
	assignedRoleAddr  = "doit_role.test"
	assignedRoleAddr2 = "doit_role.other"
	assignedUserAddr  = "doit_user.test"
)

// testAccRoleCaptureID records the ID of a role in state so a later check can
// still query it after the resource has left state.
func testAccRoleCaptureID(addr string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not found in state", addr)
		}
		*dst = rs.Primary.ID
		return nil
	}
}

// testAccRoleGone asserts the role no longer exists in the API. CheckDestroy
// only sees resources still tracked in state, so it asks the API directly.
func testAccRoleGone(t *testing.T, ids ...*string) resource.TestCheckFunc {
	t.Helper()
	return func(*terraform.State) error {
		client := getAPIClient(t)
		ctx := context.WithoutCancel(t.Context())
		for _, id := range ids {
			if *id == "" {
				continue
			}
			resp, err := client.GetRoleWithResponse(ctx, *id)
			if err != nil {
				return fmt.Errorf("getting role %s: %w", *id, err)
			}
			if resp.StatusCode() != http.StatusNotFound {
				return fmt.Errorf("role %s still exists after destroy (status %d)", *id, resp.StatusCode())
			}
		}
		return nil
	}
}

// deleteTestRole deletes a role directly through the API and fails the test
// unless the API accepts it.
func deleteTestRole(t *testing.T, id string) {
	t.Helper()
	resp, err := getAPIClient(t).DeleteRoleWithResponse(context.WithoutCancel(t.Context()), id)
	if err != nil {
		t.Fatalf("deleting role %s: %v", id, err)
	}
	if resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusOK {
		t.Fatalf("deleting role %s returned %d: %s", id, resp.StatusCode(), string(resp.Body))
	}
}

func testAccRoleAssignedConfig(roleName, email string) string {
	return fmt.Sprintf(`
resource "doit_role" "test" {
  name = %q
}

resource "doit_user" "test" {
  email   = %q
  role_id = doit_role.test.id
}
`, roleName, email)
}

func testAccRoleAssignedTwoRolesConfig(roleName, otherName, email string, useOther bool) string {
	roleRef := "doit_role.test.id"
	if useOther {
		roleRef = "doit_role.other.id"
	}
	return fmt.Sprintf(`
resource "doit_role" "test" {
  name = %q
}

resource "doit_role" "other" {
  name = %q
}

resource "doit_user" "test" {
  email   = %q
  role_id = %s
}
`, roleName, otherName, email, roleRef)
}

// TestAccRole_AssignedToInvitedUser_Destroy is the regression for #385: a role
// held by an invited user must be destroyable together with that user.
func TestAccRole_AssignedToInvitedUser_Destroy(t *testing.T) {
	email := testAccInviteEmail(t)
	rName := acctest.RandomWithPrefix("tf-acc-role-assigned")
	var roleID string

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_INVITE_EMAIL
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccRoleGone(t, &roleID),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleAssignedConfig(rName, email),
				Check:  testAccRoleCaptureID(assignedRoleAddr, &roleID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValuePairs(
						assignedUserAddr, tfjsonpath.New("role_id"),
						assignedRoleAddr, tfjsonpath.New("id"),
						compare.ValuesSame()),
				},
			},
			// Drift check.
			{
				Config: testAccRoleAssignedConfig(rName, email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccRole_UserRoleChange_Destroy moves an invited user from one role to
// another and then destroys everything. The old role was also counted as in
// use on role change, so it must be deletable afterwards.
func TestAccRole_UserRoleChange_Destroy(t *testing.T) {
	email := testAccInviteEmail(t)
	rName := acctest.RandomWithPrefix("tf-acc-role-a")
	otherName := acctest.RandomWithPrefix("tf-acc-role-b")
	var roleID, otherID string

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_INVITE_EMAIL
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccRoleGone(t, &roleID, &otherID),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleAssignedTwoRolesConfig(rName, otherName, email, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccRoleCaptureID(assignedRoleAddr, &roleID),
					testAccRoleCaptureID(assignedRoleAddr2, &otherID),
				),
			},
			{
				Config: testAccRoleAssignedTwoRolesConfig(rName, otherName, email, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValuePairs(
						assignedUserAddr, tfjsonpath.New("role_id"),
						assignedRoleAddr2, tfjsonpath.New("id"),
						compare.ValuesSame()),
				},
			},
			// Drift check.
			{
				Config: testAccRoleAssignedTwoRolesConfig(rName, otherName, email, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccRole_UserDeletedOutOfBand_Destroy reproduces the issue directly: the
// holder is deleted behind Terraform's back, then the role is destroyed.
func TestAccRole_UserDeletedOutOfBand_Destroy(t *testing.T) {
	email := testAccInviteEmail(t)
	rName := acctest.RandomWithPrefix("tf-acc-role-oob")
	var roleID string

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_INVITE_EMAIL
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccRoleGone(t, &roleID),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleAssignedConfig(rName, email),
				Check:  testAccRoleCaptureID(assignedRoleAddr, &roleID),
			},
			// The user disappears; Terraform notices and plans to re-invite.
			{
				PreConfig:          func() { deleteTestUser(t, email) },
				Config:             testAccRoleAssignedConfig(rName, email),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccRole_DeletedWhileAssigned_UserLosesRole deletes a role through the
// API while an invited user still holds it. The delete must succeed, the user
// must survive without a role, and Terraform must plan to re-create the role.
func TestAccRole_DeletedWhileAssigned_UserLosesRole(t *testing.T) {
	email := testAccInviteEmail(t)
	rName := acctest.RandomWithPrefix("tf-acc-role-del")
	var roleID string

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_INVITE_EMAIL
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccRoleGone(t, &roleID),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleAssignedConfig(rName, email),
				Check:  testAccRoleCaptureID(assignedRoleAddr, &roleID),
			},
			{
				PreConfig: func() { deleteTestRole(t, roleID) },
				Config:    testAccRoleAssignedConfig(rName, email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(assignedRoleAddr, plancheck.ResourceActionCreate),
					},
				},
				// The re-created role is wired back to the user.
				Check: testAccRoleCaptureID(assignedRoleAddr, &roleID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValuePairs(
						assignedUserAddr, tfjsonpath.New("role_id"),
						assignedRoleAddr, tfjsonpath.New("id"),
						compare.ValuesSame()),
				},
			},
			// Drift check.
			{
				Config: testAccRoleAssignedConfig(rName, email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccRole_RemovedFromConfigWhileAssigned drives the delete through the
// provider's own Delete while an invited user still holds the role: step 2
// drops the role resource but keeps the user.
func TestAccRole_RemovedFromConfigWhileAssigned(t *testing.T) {
	email := testAccInviteEmail(t)
	rName := acctest.RandomWithPrefix("tf-acc-role-rm")
	var roleID string

	deleteTestUser(t, email)
	t.Cleanup(func() { deleteTestUser(t, email) })

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_INVITE_EMAIL
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccRoleGone(t, &roleID),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleAssignedConfig(rName, email),
				Check:  testAccRoleCaptureID(assignedRoleAddr, &roleID),
			},
			{
				Config: testAccUserBasic(email),
				Check:  testAccRoleGone(t, &roleID),
			},
			// Drift check.
			{
				Config: testAccUserBasic(email),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
