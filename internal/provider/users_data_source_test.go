package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccUsersDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_users.test", "users.#"),
					resource.TestCheckResourceAttrSet("data.doit_users.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_users.test", "users.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_users.test", "users.0.email"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccUsersDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccUsersDataSource_EmailFilter(t *testing.T) {
	testUser := os.Getenv("TEST_USER")
	if testUser == "" {
		t.Skip("TEST_USER must be set for this test")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceEmailFilterConfig(testUser),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_users.by_email", "users.#", "1"),
					resource.TestCheckResourceAttr("data.doit_users.by_email", "row_count", "1"),
					resource.TestCheckResourceAttr("data.doit_users.by_email", "users.0.email", testUser),
					resource.TestCheckResourceAttrSet("data.doit_users.by_email", "users.0.id"),
					// TEST_USER has firstName, lastName, jobTitle populated
					resource.TestCheckResourceAttrSet("data.doit_users.by_email", "users.0.first_name"),
					resource.TestCheckResourceAttrSet("data.doit_users.by_email", "users.0.last_name"),
					resource.TestCheckResourceAttrSet("data.doit_users.by_email", "users.0.job_title"),
				),
			},
			// Drift verification
			{
				Config: testAccUsersDataSourceEmailFilterConfig(testUser),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccUsersDataSource_EmailFilterNullOptionals(t *testing.T) {
	testUser2 := os.Getenv("TEST_USER_2")
	if testUser2 == "" {
		t.Skip("TEST_USER_2 must be set for this test")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceEmailFilterConfig(testUser2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_users.by_email", "users.#", "1"),
					resource.TestCheckResourceAttr("data.doit_users.by_email", "row_count", "1"),
					resource.TestCheckResourceAttr("data.doit_users.by_email", "users.0.email", testUser2),
					resource.TestCheckResourceAttrSet("data.doit_users.by_email", "users.0.id"),
					// TEST_USER_2 has no firstName, lastName, jobTitle — verify they are absent
					resource.TestCheckNoResourceAttr("data.doit_users.by_email", "users.0.first_name"),
					resource.TestCheckNoResourceAttr("data.doit_users.by_email", "users.0.last_name"),
					resource.TestCheckNoResourceAttr("data.doit_users.by_email", "users.0.job_title"),
				),
			},
			// Drift verification
			{
				Config: testAccUsersDataSourceEmailFilterConfig(testUser2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccUsersDataSourceConfig() string {
	return `
data "doit_users" "test" {}
`
}

func testAccUsersDataSourceEmailFilterConfig(email string) string {
	return fmt.Sprintf(`
data "doit_users" "by_email" {
  email = %q
}
`, email)
}
