package provider_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testAccServiceAccountTokenAddr = "doit_service_account_token.this"

// Each test creates its own service account because a service account holds at
// most 10 tokens and tests run in parallel.

func TestAccServiceAccountToken_Lifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat")

	driftCheck := func(cfg string) resource.TestStep {
		return resource.TestStep{
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		}
	}

	active := testAccServiceAccountTokenConfig(rName, "", "")
	disabled := testAccServiceAccountTokenConfig(rName, "", `state = "disabled"`)
	// state is Optional+Computed and not clearable, so re-enabling must set it explicitly.
	reenabled := testAccServiceAccountTokenConfig(rName, "", `state = "active"`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with expires_time omitted (server default)
			{
				Config: active,
				// The server picks the expiry when it is omitted: one year after creation.
				Check: resource.TestCheckResourceAttrWith(testAccServiceAccountTokenAddr, "expires_time", func(v string) error {
					expires, err := time.Parse(time.RFC3339, v)
					if err != nil {
						return fmt.Errorf("expires_time %q is not RFC 3339: %w", v, err)
					}
					if d := time.Until(expires); d < 364*24*time.Hour || d > 366*24*time.Hour {
						return fmt.Errorf("default expires_time %q is %s away, want about one year", v, d)
					}
					return nil
				}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccServiceAccountTokenAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("state"), knownvalue.StringExact("active")),
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("customer_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("create_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("expires_time"), knownvalue.NotNull()),
					statecheck.ExpectSensitiveValue(testAccServiceAccountTokenAddr, tfjsonpath.New("access_token")),
				},
			},
			// Step 2: Drift check
			driftCheck(active),
			// Step 3: Disable in place
			{
				Config: disabled,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccServiceAccountTokenAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("state"), knownvalue.StringExact("disabled")),
				},
			},
			// Step 4: Drift check on a disabled token (plan/apply/refresh)
			driftCheck(disabled),
			// Step 5: Re-enable in place
			{
				Config: reenabled,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccServiceAccountTokenAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("state"), knownvalue.StringExact("active")),
				},
			},
			// Step 6: Drift check
			driftCheck(reenabled),
		},
	})
}

func TestAccServiceAccountToken_ExplicitExpiresTime(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-exp")
	cfg := testAccServiceAccountTokenConfig(rName, "", `expires_time = "2099-01-01T00:00:00Z"`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("expires_time"), knownvalue.StringExact("2099-01-01T00:00:00Z")),
				},
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// The API accepts the whole timestamp range; the extremes must round-trip
// without drift.
func TestAccServiceAccountToken_ExpiresTimeBoundaries(t *testing.T) {
	for name, value := range map[string]string{
		"far_future": "9999-12-31T23:59:59Z",
		"epoch":      "1970-01-01T00:00:00Z",
	} {
		t.Run(name, func(t *testing.T) {
			cfg := testAccServiceAccountTokenConfig(acctest.RandomWithPrefix("tf-acc-sat-"+name), "", fmt.Sprintf("expires_time = %q", value))

			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: cfg,
						ConfigStateChecks: []statecheck.StateCheck{
							statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("expires_time"), knownvalue.StringExact(value)),
						},
					},
					{
						Config: cfg,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}

func TestAccServiceAccountToken_DisabledOnCreate(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-dis")
	cfg := testAccServiceAccountTokenConfig(rName, "", `state = "disabled"`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("state"), knownvalue.StringExact("disabled")),
				},
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccServiceAccountToken_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-imp")
	cfg := testAccServiceAccountTokenConfig(rName, "", "")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: cfg},
			// access_token is only returned by create, so it is null after import.
			{
				ResourceName:            testAccServiceAccountTokenAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"access_token", "last_used_time"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[testAccServiceAccountTokenAddr]
					if !ok {
						return "", fmt.Errorf("resource %s not found in state", testAccServiceAccountTokenAddr)
					}
					return rs.Primary.Attributes["service_account_id"] + "/" + rs.Primary.ID, nil
				},
			},
			// Drift check after import
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// A malformed import ID must fail with a clear error. The failed import step
// leaves the state of step 1 untouched, so the post-test destroy deletes the
// service account using the ETag the API returned from create. That is a
// regression test for create responses returning an ETag that GET does not
// (CMP-53578), which made the delete fail with 412.
func TestAccServiceAccountToken_ImportInvalidID(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-badimp")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: testAccServiceAccountTokenConfig(rName, "", "")},
			{
				ResourceName:  testAccServiceAccountTokenAddr,
				ImportState:   true,
				ImportStateId: "not-a-composite-id",
				ExpectError:   regexp.MustCompile(`service_account_id/token_id`),
			},
		},
	})
}

func TestAccServiceAccountToken_ReplaceOnNameChange(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-rep")

	// The first and second access tokens must differ: replacement mints a new credential.
	var firstToken string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountTokenConfig(rName, "", ""),
				Check: resource.TestCheckResourceAttrWith(testAccServiceAccountTokenAddr, "access_token", func(v string) error {
					firstToken = v
					return nil
				}),
			},
			{
				Config: testAccServiceAccountTokenConfig(rName, rName+"-renamed", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccServiceAccountTokenAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttrWith(testAccServiceAccountTokenAddr, "access_token", func(v string) error {
					if v == "" || v == firstToken {
						return fmt.Errorf("expected a new access_token after replacement")
					}
					return nil
				}),
			},
			{
				Config: testAccServiceAccountTokenConfig(rName, rName+"-renamed", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// A token created with a past expires_time is expired immediately. Disabling it
// must persist without drift, and re-enabling must fail.
func TestAccServiceAccountToken_Expired(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-old")
	expired := `expires_time = "2020-01-01T00:00:00Z"`
	expiredDisabled := expired + "\n  state = \"disabled\""

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: testAccServiceAccountTokenConfig(rName, "", expired)},
			{
				Config: testAccServiceAccountTokenConfig(rName, "", expiredDisabled),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccServiceAccountTokenAddr, tfjsonpath.New("state"), knownvalue.StringExact("disabled")),
				},
			},
			{
				Config: testAccServiceAccountTokenConfig(rName, "", expiredDisabled),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Re-enabling an expired token is refused by the API.
			{
				Config:      testAccServiceAccountTokenConfig(rName, "", expired+"\n  state = \"active\""),
				ExpectError: regexp.MustCompile(`token_expired`),
			},
		},
	})
}

func TestAccServiceAccountToken_InvalidExpiresTime(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-fmt")

	for name, value := range map[string]string{
		"offset":     "2027-06-01T10:00:00+02:00",
		"fractional": "2027-06-01T08:00:00.123Z",
	} {
		t.Run(name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config:      testAccServiceAccountTokenConfig(rName+"-"+name, "", fmt.Sprintf("expires_time = %q", value)),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(`RFC 3339 timestamp in UTC with whole\s+seconds`),
					},
				},
			})
		})
	}
}

// testAccServiceAccountTokenConfig returns a service account plus one token on
// it. tokenName overrides the token name (default: same as the service
// account); extra is spliced into the token block.
func testAccServiceAccountTokenConfig(name, tokenName, extra string) string {
	if tokenName == "" {
		tokenName = name
	}
	return fmt.Sprintf(`
resource "doit_service_account" "this" {
  name = %q
}

resource "doit_service_account_token" "this" {
  service_account_id = doit_service_account.this.id
  name               = %q
  %s
}
`, name, tokenName, extra)
}
