package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testAccCloudflowConnectionAddr = "doit_cloudflow_connection.this"

// The connections are created without any cloud-side deployment: the API accepts
// AWS account IDs and role names it cannot reach, and GCP connections only need a
// project the DoiT customer knows (TEST_PROJECT).
//
// Every configuration lists TEST_USER as the only owner. The user behind the API
// token has to be a collaborator, otherwise the API denies access to the
// connection it just created and nobody can delete it again, so no test changes
// the owner or sends a collaborator list without it.

// testAccCloudflowAWSAccountID returns a unique, well-formed AWS account ID.
func testAccCloudflowAWSAccountID() string {
	return fmt.Sprintf("999%09d", acctest.RandIntRange(100000000, 999999999))
}

// testAccCloudflowGCPServiceAccount returns a unique service account name.
func testAccCloudflowGCPServiceAccount() string {
	return fmt.Sprintf("tfacc-%08d", acctest.RandIntRange(10000000, 99999999))
}

func testAccCloudflowOwner() string {
	return fmt.Sprintf(`{ email = %q, role = "owner" }`, testUser())
}

func testAccCloudflowConnection(body string) string {
	return fmt.Sprintf(`
resource "doit_cloudflow_connection" "this" {
%s
}
`, body)
}

func testAccCloudflowConnectionDriftCheck(cfg string) resource.TestStep {
	return resource.TestStep{
		Config: cfg,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}

func testAccCloudflowConnectionImportStep() resource.TestStep {
	return resource.TestStep{
		ResourceName:                         testAccCloudflowConnectionAddr,
		ImportState:                          true,
		ImportStateVerify:                    true,
		ImportStateVerifyIdentifierAttribute: "connection_id",
		ImportStateIdFunc:                    testAccCloudflowConnectionImportID,
		ImportStateVerifyIgnore:              []string{"timeouts"},
	}
}

func testAccCloudflowConnectionImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources[testAccCloudflowConnectionAddr]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", testAccCloudflowConnectionAddr)
	}
	return rs.Primary.Attributes["connection_id"], nil
}

func testAccCheckCloudflowConnectionDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getAPIClient(t)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "doit_cloudflow_connection" {
				continue
			}
			id := rs.Primary.Attributes["connection_id"]
			resp, err := client.GetCloudflowConnectionWithResponse(t.Context(), id)
			if err != nil {
				return fmt.Errorf("error checking CloudFlow connection %s: %w", id, err)
			}
			if resp.StatusCode() != 404 {
				return fmt.Errorf("CloudFlow connection %s still exists (status %d)", id, resp.StatusCode())
			}
		}
		return nil
	}
}

func testAccCloudflowAWSConfig(name, account, role, extra string) string {
	return testAccCloudflowConnection(fmt.Sprintf(`
  name          = %[1]q
  collaborators = [%[4]s]
  aws_config = {
    role_name = %[3]q
    context   = [{ account_id = %[2]q }]
%[5]s
  }
`, name, account, role, testAccCloudflowOwner(), extra))
}

func testAccCloudflowGCPConfig(name, project, serviceAccount, extra string) string {
	return testAccCloudflowConnection(fmt.Sprintf(`
  name          = %[1]q
  collaborators = [%[4]s]
  gcp_config = {
    level                = "project"
    project_id           = %[2]q
    service_account_name = %[3]q
%[5]s
  }
`, name, project, serviceAccount, testAccCloudflowOwner(), extra))
}

func TestAccCloudflowConnection_AWS_Lifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	account := testAccCloudflowAWSAccountID()
	secondAccount := testAccCloudflowAWSAccountID()
	role := acctest.RandomWithPrefix("tfacc-role")

	create := testAccCloudflowConnection(fmt.Sprintf(`
  name        = %[1]q
  description = "created by the acceptance tests"
  collaborators = [%[4]s]
  aws_config = {
    role_name = %[3]q
    context   = [{ account_id = %[2]q, regions = ["us-east-1", "eu-west-1"] }]
    permissions = {
      aws_managed_policies = ["arn:aws:iam::aws:policy/ReadOnlyAccess"]
      actions              = [{ value = "s3:ListBucket" }, { value = "ec2:DescribeInstances" }]
    }
    scope_explicit_account_ids = [%[2]q]
  }
  timeouts = {
    create = "10m"
    read   = "5m"
    update = "10m"
    delete = "10m"
  }
`, rName, account, role, testAccCloudflowOwner()))

	// Renamed, with a second account, reordered and extended permissions, and management settings.
	update := testAccCloudflowConnection(fmt.Sprintf(`
  name        = "%[1]s-renamed"
  description = "updated by the acceptance tests"
  collaborators = [%[5]s]
  aws_config = {
    role_name = %[3]q
    context = [
      { account_id = %[2]q, regions = ["us-east-1"] },
      { account_id = %[4]q },
    ]
    permissions = {
      aws_managed_policies = ["arn:aws:iam::aws:policy/ReadOnlyAccess", "arn:aws:iam::aws:policy/SecurityAudit"]
      actions              = [{ value = "ec2:DescribeInstances" }, { value = "s3:ListBucket" }, { value = "iam:ListRoles" }]
    }
    management_account                         = "111111111111"
    organization_root_id                       = "r-ab12"
    scope_explicit_account_ids                 = [%[2]q, %[4]q]
    scope_excluded_account_ids                 = ["222222222222"]
    scope_management_account_explicit_in_scope = true
  }
`, rName, account, role, secondAccount, testAccCloudflowOwner()))

	// Everything optional removed again: replace semantics must clear it.
	cleared := testAccCloudflowAWSConfig(rName+"-renamed", account, role, "")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with permissions and scope
			{
				Config: create,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("connection_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("description"), knownvalue.StringExact("created by the acceptance tests")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("status"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("etag"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("updated_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("role_name"), knownvalue.StringExact(role)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(0).AtMapKey("account_id"), knownvalue.StringExact(account)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(0).AtMapKey("status"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(0).AtMapKey("regions"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("us-east-1"), knownvalue.StringExact("eu-west-1"),
					})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("permissions").AtMapKey("aws_managed_policies"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:iam::aws:policy/ReadOnlyAccess"),
					})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("permissions").AtMapKey("actions").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("s3:ListBucket")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("permissions").AtMapKey("actions").AtSliceIndex(1).AtMapKey("value"), knownvalue.StringExact("ec2:DescribeInstances")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_excluded_account_ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_explicit_account_ids"), knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(account)})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_management_account_explicit_in_scope"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("collaborators"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"email": knownvalue.StringRegexp(regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(testUser()) + `$`)),
							"role":  knownvalue.StringExact("owner"),
						}),
					})),
				},
			},
			// Step 2: Drift check
			testAccCloudflowConnectionDriftCheck(create),
			// Step 3: Import
			testAccCloudflowConnectionImportStep(),
			// Step 4: Update in place
			{
				Config: update,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-renamed")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("description"), knownvalue.StringExact("updated by the acceptance tests")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(1).AtMapKey("account_id"), knownvalue.StringExact(secondAccount)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(1).AtMapKey("regions"), knownvalue.ListExact([]knownvalue.Check{})),
					// The order of the actions is kept.
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("permissions").AtMapKey("actions").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("ec2:DescribeInstances")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("permissions").AtMapKey("actions").AtSliceIndex(2).AtMapKey("value"), knownvalue.StringExact("iam:ListRoles")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("management_account"), knownvalue.StringExact("111111111111")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("organization_root_id"), knownvalue.StringExact("r-ab12")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_excluded_account_ids"), knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("222222222222")})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_management_account_explicit_in_scope"), knownvalue.Bool(true)),
				},
			},
			// Step 5: Drift check after the update
			testAccCloudflowConnectionDriftCheck(update),
			// Step 6: Remove everything optional; the replace semantics clear it
			{
				Config: cleared,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(0).AtMapKey("regions"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("permissions"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("management_account"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("organization_root_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_explicit_account_ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_excluded_account_ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_management_account_explicit_in_scope"), knownvalue.Bool(false)),
				},
			},
			// Step 7: Drift check after clearing
			testAccCloudflowConnectionDriftCheck(cleared),
		},
	})
}

// Only the required fields: everything else resolves to its default and stays stable.
func TestAccCloudflowConnection_AWS_Minimal(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	cfg := testAccCloudflowAWSConfig(rName, testAccCloudflowAWSAccountID(), acctest.RandomWithPrefix("tfacc-role"), "")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("permissions"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("management_account"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("organization_root_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(0).AtMapKey("regions"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_excluded_account_ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_explicit_account_ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_targeted_organizational_unit_ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_management_account_explicit_in_scope"), knownvalue.Bool(false)),
				},
			},
			testAccCloudflowConnectionDriftCheck(cfg),
			testAccCloudflowConnectionImportStep(),
			testAccCloudflowConnectionDriftCheck(cfg),
		},
	})
}

// Explicit empty lists are the same as omitted ones.
func TestAccCloudflowConnection_AWS_EmptyLists(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	cfg := testAccCloudflowConnection(fmt.Sprintf(`
  name          = %[1]q
  collaborators = [%[4]s]
  aws_config = {
    role_name = %[3]q
    context   = [{ account_id = %[2]q, regions = [] }]
    scope_excluded_account_ids             = []
    scope_explicit_account_ids             = []
    scope_targeted_organizational_unit_ids = []
  }
`, rName, testAccCloudflowAWSAccountID(), acctest.RandomWithPrefix("tfacc-role"), testAccCloudflowOwner()))

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("context").AtSliceIndex(0).AtMapKey("regions"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config").AtMapKey("scope_excluded_account_ids"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			testAccCloudflowConnectionDriftCheck(cfg),
		},
	})
}

func TestAccCloudflowConnection_GCP_Lifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	serviceAccount := testAccCloudflowGCPServiceAccount()
	customRole := fmt.Sprintf("tfacc_role_%d", acctest.RandIntRange(1000000, 9999999))

	create := testAccCloudflowGCPConfig(rName, testProject(), serviceAccount, fmt.Sprintf(`
    predefined_roles = ["roles/viewer"]
    custom_role = {
      role_id     = %q
      permissions = ["compute.instances.list", "compute.instances.get"]
    }
`, customRole))

	update := testAccCloudflowConnection(fmt.Sprintf(`
  name        = "%[1]s-renamed"
  description = "gcp connection"
  collaborators = [%[4]s]
  gcp_config = {
    level                = "project"
    project_id           = %[2]q
    service_account_name = %[3]q
    predefined_roles     = ["roles/viewer", "roles/browser"]
    custom_role = {
      role_id     = %[5]q
      permissions = ["compute.instances.list"]
    }
  }
`, rName, testProject(), serviceAccount, testAccCloudflowOwner(), customRole))

	// The custom role and the predefined roles removed again.
	cleared := testAccCloudflowGCPConfig(rName+"-renamed", testProject(), serviceAccount, "")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: create,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("connection_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("level"), knownvalue.StringExact("project")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("project_id"), knownvalue.StringExact(testProject())),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("service_account_name"), knownvalue.StringExact(serviceAccount)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("organization_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("folder_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("predefined_roles"), knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("roles/viewer")})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("custom_role").AtMapKey("role_id"), knownvalue.StringExact(customRole)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("custom_role").AtMapKey("permissions"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("compute.instances.list"), knownvalue.StringExact("compute.instances.get"),
					})),
				},
			},
			testAccCloudflowConnectionDriftCheck(create),
			testAccCloudflowConnectionImportStep(),
			// Step 4: Update in place
			{
				Config: update,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-renamed")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("description"), knownvalue.StringExact("gcp connection")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("predefined_roles"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("roles/viewer"), knownvalue.StringExact("roles/browser"),
					})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("custom_role").AtMapKey("permissions"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("compute.instances.list"),
					})),
				},
			},
			testAccCloudflowConnectionDriftCheck(update),
			// Step 6: Remove the roles and the description
			{
				Config: cleared,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("predefined_roles"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("custom_role"), knownvalue.Null()),
				},
			},
			testAccCloudflowConnectionDriftCheck(cleared),
			testAccCloudflowConnectionImportStep(),
		},
	})
}

func TestAccCloudflowConnection_GCP_Minimal(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	cfg := testAccCloudflowGCPConfig(rName, testProject(), testAccCloudflowGCPServiceAccount(), "")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("predefined_roles"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("custom_role"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("organization_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("folder_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("infra_manager_project"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("enabled"), knownvalue.Bool(true)),
				},
			},
			testAccCloudflowConnectionDriftCheck(cfg),
			testAccCloudflowConnectionImportStep(),
			testAccCloudflowConnectionDriftCheck(cfg),
		},
	})
}

func TestAccCloudflowConnection_GCP_InfraManager(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	cfg := testAccCloudflowGCPConfig(rName, testProject(), testAccCloudflowGCPServiceAccount(), fmt.Sprintf(`
    infra_manager_project         = %q
    infra_manager_location        = "us-central1"
    infra_manager_service_account = "tfacc-infra"
`, testProject()))

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("infra_manager_project"), knownvalue.StringExact(testProject())),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("infra_manager_location"), knownvalue.StringExact("us-central1")),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("infra_manager_service_account"), knownvalue.StringExact("tfacc-infra")),
				},
			},
			testAccCloudflowConnectionDriftCheck(cfg),
			testAccCloudflowConnectionImportStep(),
		},
	})
}

// A connection can be created disabled and toggled in place.
func TestAccCloudflowConnection_Enabled(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	account := testAccCloudflowAWSAccountID()
	role := acctest.RandomWithPrefix("tfacc-role")
	disabled := testAccCloudflowConnection(fmt.Sprintf(`
  name          = %[1]q
  enabled       = false
  collaborators = [%[4]s]
  aws_config = {
    role_name = %[3]q
    context   = [{ account_id = %[2]q }]
  }
`, rName, account, role, testAccCloudflowOwner()))
	enabled := testAccCloudflowAWSConfig(rName, account, role, "")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: disabled,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("enabled"), knownvalue.Bool(false)),
				},
			},
			testAccCloudflowConnectionDriftCheck(disabled),
			{
				Config: enabled,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("enabled"), knownvalue.Bool(true)),
				},
			},
			testAccCloudflowConnectionDriftCheck(enabled),
			{
				Config: disabled,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("enabled"), knownvalue.Bool(false)),
				},
			},
			testAccCloudflowConnectionDriftCheck(disabled),
		},
	})
}

// The API cannot switch a connection between AWS and GCP in place, so the switch replaces it.
func TestAccCloudflowConnection_KindSwitchReplaces(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	aws := testAccCloudflowAWSConfig(rName, testAccCloudflowAWSAccountID(), acctest.RandomWithPrefix("tfacc-role"), "")
	gcp := testAccCloudflowGCPConfig(rName, testProject(), testAccCloudflowGCPServiceAccount(), "")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: aws},
			{
				Config: gcp,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("aws_config"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("gcp_config").AtMapKey("project_id"), knownvalue.StringExact(testProject())),
				},
			},
			testAccCloudflowConnectionDriftCheck(gcp),
			{
				Config: aws,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
			testAccCloudflowConnectionDriftCheck(aws),
		},
	})
}

// Collaborators other than the owner can be added and removed in place.
func TestAccCloudflowConnection_Collaborators(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	account := testAccCloudflowAWSAccountID()
	role := acctest.RandomWithPrefix("tfacc-role")
	withCollaborators := func(extra string) string {
		return testAccCloudflowConnection(fmt.Sprintf(`
  name          = %[1]q
  collaborators = [%[4]s%[5]s]
  aws_config = {
    role_name = %[3]q
    context   = [{ account_id = %[2]q }]
  }
`, rName, account, role, testAccCloudflowOwner(), extra))
	}
	ownerOnly := withCollaborators("")
	withEditor := withCollaborators(fmt.Sprintf(`, { email = %q, role = "editor" }`, testUser2()))
	withEditorAndUser := withCollaborators(fmt.Sprintf(`, { email = %q, role = "user" }`, testUser2()))

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: ownerOnly,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("collaborators"), knownvalue.ListSizeExact(1)),
				},
			},
			testAccCloudflowConnectionDriftCheck(ownerOnly),
			{
				Config: withEditor,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCloudflowConnectionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("collaborators"), knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("collaborators").AtSliceIndex(1).AtMapKey("role"), knownvalue.StringExact("editor")),
				},
			},
			testAccCloudflowConnectionDriftCheck(withEditor),
			testAccCloudflowConnectionImportStep(),
			{
				Config: withEditorAndUser,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("collaborators").AtSliceIndex(1).AtMapKey("role"), knownvalue.StringExact("user")),
				},
			},
			testAccCloudflowConnectionDriftCheck(withEditorAndUser),
			{
				Config: ownerOnly,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCloudflowConnectionAddr, tfjsonpath.New("collaborators"), knownvalue.ListSizeExact(1)),
				},
			},
			testAccCloudflowConnectionDriftCheck(ownerOnly),
		},
	})
}

// Invalid configurations are rejected at plan time, before any API call.
func TestAccCloudflowConnection_Validation(t *testing.T) {
	owner := testAccCloudflowOwner()
	aws := `aws_config = { role_name = "r", context = [{ account_id = "123456789012" }] }`
	gcp := `gcp_config = { level = "project", project_id = "p", service_account_name = "sa" }`
	config := func(lines ...string) string {
		body := "  name = \"tf-acc-cf-invalid\"\n"
		for _, l := range lines {
			body += "  " + l + "\n"
		}
		return testAccCloudflowConnection(body)
	}
	withOwner := func(lines ...string) string {
		return config(append([]string{"collaborators = [" + owner + "]"}, lines...)...)
	}

	tests := []struct {
		name   string
		config string
		error  string
	}{
		{"neither configuration", withOwner(), `(?s)Exactly one of these attributes`},
		{"both configurations", withOwner(aws, gcp), `(?s)Invalid Attribute Combination|Exactly one of these attributes`},
		{"no collaborators", config(aws), `collaborators`},
		{"empty collaborators", config("collaborators = []", aws), `Missing owner`},
		{"no owner", config(`collaborators = [{ email = "a@example.com", role = "editor" }]`, aws), `Missing owner`},
		{"two owners", config(`collaborators = [`+owner+`, { email = "b@example.com", role = "owner" }]`, aws), `Too many owners`},
		{"duplicate collaborator", config(`collaborators = [`+owner+`, { email = "`+testUser()+`", role = "editor" }]`, aws), `Duplicate collaborator`},
		{"invalid email", config(`collaborators = [{ email = "not-an-email", role = "owner" }]`, aws), `plain\s+email\s+address`},
		{"email with display name", config(`collaborators = [{ email = "Jane <jane@example.com>", role = "owner" }]`, aws), `plain\s+email\s+address`},
		{"invalid role", config(`collaborators = [{ email = "a@example.com", role = "admin" }]`, aws), `owner.*editor.*user`},
		{"gcp level without matching id", withOwner(`gcp_config = { level = "folder", project_id = "p", service_account_name = "sa" }`), `folder_id\s+is\s+required`},
		{"gcp organization without id", withOwner(`gcp_config = { level = "organization", service_account_name = "sa" }`), `organization_id\s+is\s+required`},
		{"gcp invalid level", withOwner(`gcp_config = { level = "region", project_id = "p", service_account_name = "sa" }`), `owner|organization|folder|project`},
		{"gcp without service account", withOwner(`gcp_config = { level = "project", project_id = "p" }`), `service_account_name`},
		{"gcp empty project id", withOwner(`gcp_config = { level = "project", project_id = "", service_account_name = "sa" }`), `at\s+least\s+1`},
		{"gcp custom role without id", withOwner(`gcp_config = { level = "project", project_id = "p", service_account_name = "sa", custom_role = { permissions = ["a.b"] } }`), `role_id`},
		{"aws without role name", withOwner(`aws_config = { context = [{ account_id = "123456789012" }] }`), `role_name`},
		{"aws without context", withOwner(`aws_config = { role_name = "r" }`), `context`},
		{"aws empty context", withOwner(`aws_config = { role_name = "r", context = [] }`), `at\s+least\s+1`},
		{"aws invalid account id", withOwner(`aws_config = { role_name = "r", context = [{ account_id = "12345" }] }`), `12-digit`},
		{"aws empty permissions block", withOwner(`aws_config = { role_name = "r", context = [{ account_id = "123456789012" }], permissions = {} }`), `Empty permissions`},
		{"aws permissions with empty lists", withOwner(`aws_config = { role_name = "r", context = [{ account_id = "123456789012" }], permissions = { actions = [], aws_managed_policies = [] } }`), `Empty permissions`},
		{"aws empty action", withOwner(`aws_config = { role_name = "r", context = [{ account_id = "123456789012" }], permissions = { actions = [{ value = "" }] } }`), `at\s+least\s+1`},
		{"empty name", testAccCloudflowConnection(`  name = ""` + "\n  collaborators = [" + owner + "]\n  " + aws), `at\s+least\s+1`},
		{"empty description", withOwner(aws, `description = ""`), `at\s+least\s+1`},
	}

	steps := make([]resource.TestStep, 0, len(tests))
	for _, tt := range tests {
		steps = append(steps, resource.TestStep{
			Config:      tt.config,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)` + tt.error),
		})
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps:                    steps,
	})
}

// The API rejects a second connection that uses the same role name in the same account.
// The first connection is created and cleaned up; the second one fails readably.
func TestAccCloudflowConnection_DuplicateRoleName(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	account := testAccCloudflowAWSAccountID()
	role := acctest.RandomWithPrefix("tfacc-role")
	cfg := fmt.Sprintf(`
resource "doit_cloudflow_connection" "this" {
  name          = %[1]q
  collaborators = [%[4]s]
  aws_config = {
    role_name = %[3]q
    context   = [{ account_id = %[2]q }]
  }
}

resource "doit_cloudflow_connection" "duplicate" {
  depends_on    = [doit_cloudflow_connection.this]
  name          = "%[1]s-duplicate"
  collaborators = [%[4]s]
  aws_config = {
    role_name = %[3]q
    context   = [{ account_id = %[2]q }]
  }
}
`, rName, account, role, testAccCloudflowOwner())

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?s)Error Creating CloudFlow Connection.*already exists`),
			},
		},
	})
}

// A connection deleted outside Terraform is planned for re-creation.
func TestAccCloudflowConnection_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cf")
	cfg := testAccCloudflowAWSConfig(rName, testAccCloudflowAWSAccountID(), acctest.RandomWithPrefix("tfacc-role"), "")
	var connectionID string

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.TestCheckResourceAttrWith(testAccCloudflowConnectionAddr, "connection_id", func(value string) error {
					if value == "" {
						return fmt.Errorf("connection_id is empty")
					}
					connectionID = value
					return nil
				}),
			},
			{
				PreConfig: func() {
					client := getAPIClient(t)
					get, err := client.GetCloudflowConnectionWithResponse(t.Context(), connectionID)
					if err != nil || get.JSON200 == nil || get.JSON200.Etag == nil {
						t.Fatalf("Failed to read the connection for its etag: %v (%v)", err, get)
					}
					resp, err := client.DeleteCloudflowConnectionWithResponse(t.Context(), connectionID, &models.DeleteCloudflowConnectionParams{IfMatch: *get.JSON200.Etag})
					if err != nil {
						t.Fatalf("Failed to delete the connection via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204, or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect the deletion and plan to recreate
			},
		},
	})
}

func TestAccCloudflowConnection_ImportNotFound(t *testing.T) {
	cfg := testAccCloudflowAWSConfig(acctest.RandomWithPrefix("tf-acc-cf"), testAccCloudflowAWSAccountID(), acctest.RandomWithPrefix("tfacc-role"), "")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:        cfg,
				ResourceName:  testAccCloudflowConnectionAddr,
				ImportState:   true,
				ImportStateId: "does-not-exist",
				ExpectError:   regexp.MustCompile(`Cannot import non-existent remote object`),
			},
		},
	})
}

// The shipped example applies, stays stable, and cleans up. Its placeholders are
// replaced by values that exist in the test environment; the owner is TEST_USER so
// that the API token's user can still reach the connections.
func TestAccCloudflowConnection_Example(t *testing.T) {
	raw, err := os.ReadFile("../../examples/resources/doit_cloudflow_connection/resource.tf")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	// The harness supplies the provider itself.
	cfg := regexp.MustCompile(`(?s)^terraform \{.*?\n\}\n`).ReplaceAllString(string(raw), "")

	suffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	cfg = strings.NewReplacer(
		"owner@example.com", testUser(),
		"engineer@example.com", testUser2(),
		"my-production-project", testProject(),
		`service_account_name = "doit-cloudflow"`, `service_account_name = "`+testAccCloudflowGCPServiceAccount()+`"`,
		"doitCloudflowOperator", "tfaccExample"+suffix,
		`role_name = "doit-cloudflow-paused"`, `role_name = "tfacc-paused-`+suffix+`"`,
		`role_name = "doit-cloudflow"`, `role_name = "tfacc-example-`+suffix+`"`,
		"123456789012", testAccCloudflowAWSAccountID(),
		"210987654321", testAccCloudflowAWSAccountID(),
		"111111111111", testAccCloudflowAWSAccountID(),
	).Replace(cfg)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCloudflowConnectionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_cloudflow_connection.gcp", tfjsonpath.New("gcp_config").AtMapKey("level"), knownvalue.StringExact("project")),
					statecheck.ExpectKnownValue("doit_cloudflow_connection.aws", tfjsonpath.New("aws_config").AtMapKey("context"), knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue("doit_cloudflow_connection.disabled", tfjsonpath.New("enabled"), knownvalue.Bool(false)),
				},
			},
			testAccCloudflowConnectionDriftCheck(cfg),
		},
	})
}
