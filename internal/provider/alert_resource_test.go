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

func TestAccAlert(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Alert Create
			{
				Config: testAccAlert(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_alert.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-alert-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("time_interval"),
						knownvalue.StringExact("month")),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("value"),
						knownvalue.Float64Exact(1000)),
				},
			},
			// Test Alert Update (In-place)
			{
				Config: testAccAlertUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_alert.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-alert-updated-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("value"),
						knownvalue.Float64Exact(2000)),
				},
			},
			// Step 3: Drift check — re-apply same updated config, expect no changes.
			{
				Config: testAccAlertUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccAlert_Import(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlert(n),
			},
			{
				ResourceName:      "doit_alert.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Drift check — re-apply config after import, expect no changes.
			{
				Config: testAccAlert(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccAlert_WithScopes(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithScopes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("fixed"),
								"id":     knownvalue.StringExact("cloud_provider"),
								"mode":   knownvalue.StringExact("is"),
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("amazon-web-services")}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccAlertWithScopes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAlert_WithEmptyScopes tests that explicitly setting scopes = [] works correctly.
// This verifies the fix for state inconsistency between empty list and null.
func TestAccAlert_WithEmptyScopes(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithEmptyScopes(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListExact([]knownvalue.Check{})), // Empty list, not null
				},
			},
		},
	})
}

// TestAccAlert_WithAttributions tests the deprecated attributions field for backward compatibility.
func TestAccAlert_WithAttributions(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithAttributions(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("attributions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact(testAttribution()),
						})),
				},
			},
		},
	})
}

// TestAccAlert_WithEmptyAttributions tests that explicitly setting attributions = [] works correctly.
// This verifies the fix for state inconsistency between empty list and null.
func TestAccAlert_WithEmptyAttributions(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithEmptyAttributions(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("attributions"),
						knownvalue.ListExact([]knownvalue.Check{})), // Empty list, not null
				},
			},
		},
	})
}

// TestAccAlert_WithEmptyRecipients documents that recipients = [] is not allowed.
// The API adds the creator as a default recipient when recipients is empty, causing state drift.
// To prevent this, the provider validates that at least one recipient is specified.
func TestAccAlert_WithEmptyRecipients(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAlertWithEmptyRecipients(n),
				ExpectError: regexp.MustCompile(`At Least One Recipient Required`),
			},
		},
	})
}

// TestAccAlert_WithInverseScope tests scope exclusion filters.
func TestAccAlert_WithInverseScope(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithInverseScope(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":    knownvalue.StringExact("fixed"),
								"id":      knownvalue.StringExact("cloud_provider"),
								"mode":    knownvalue.StringExact("is"),
								"inverse": knownvalue.Bool(true),
								"values":  knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("google-cloud-platform")}),
							}),
						}),
					),
				},
			},
		},
	})
}

// TestAccAlert_WithEvaluateForEach tests dimension breakdown evaluation.
func TestAccAlert_WithEvaluateForEach(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithEvaluateForEach(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("evaluate_for_each"),
						knownvalue.StringExact("fixed:service_description")),
				},
			},
		},
	})
}

// TestAccAlert_PercentageChange tests the percentage-change condition.
func TestAccAlert_PercentageChange(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertPercentageChange(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("condition"),
						knownvalue.StringExact("percentage-change")),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("value"),
						knownvalue.Float64Exact(50)),
				},
			},
		},
	})
}

// TestAccAlert_DifferentOperators tests various operator values.
func TestAccAlert_DifferentOperators(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test with "lt" operator (less than)
			{
				Config: testAccAlertWithOperator(n, "lt"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("operator"),
						knownvalue.StringExact("lt")),
				},
			},
		},
	})
}

// TestAccAlert_DifferentTimeIntervals tests various time_interval values.
func TestAccAlert_DifferentTimeIntervals(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test with "day" interval
			{
				Config: testAccAlertWithTimeInterval(n, "day"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("time_interval"),
						knownvalue.StringExact("day")),
				},
			},
			// Update to "week" interval
			{
				Config: testAccAlertWithTimeInterval(n, "week"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("time_interval"),
						knownvalue.StringExact("week")),
				},
			},
		},
	})
}

// TestAccAlert_NoRecipients tests alerts without explicit recipients (defaults to creator).
func TestAccAlert_NoRecipients(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertNoRecipients(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// API will default to creator's email
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("recipients"),
						knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

// TestAccAlert_AllConfigAttributes tests an alert with all optional config attributes.
func TestAccAlert_AllConfigAttributes(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertAllConfigAttributes(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("condition"),
						knownvalue.StringExact("value")),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("currency"),
						knownvalue.StringExact("EUR")),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("operator"),
						knownvalue.StringExact("gt")),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("time_interval"),
						knownvalue.StringExact("week")),
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("evaluate_for_each"),
						knownvalue.StringExact("fixed:cloud_provider")),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccAlertAllConfigAttributes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlert(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-%d"
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
`, i)
}

func testAccAlertUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-updated-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 2000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}
`, i)
}

func testAccAlertWithScopes(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-scopes-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "fixed"
        id     = "cloud_provider"
        mode   = "is"
        values = ["amazon-web-services"]
      }
    ]
  }
}
`, i)
}

func testAccAlertWithEmptyScopes(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-empty-scopes-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes        = []
  }
}
`, i)
}

func testAccAlertWithAttributions(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-attributions-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    attributions  = ["%s"]
  }
}
`, i, testAttribution())
}

func testAccAlertWithEmptyAttributions(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-empty-attributions-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    attributions  = []
  }
}
`, i)
}

func testAccAlertWithEmptyRecipients(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-empty-recipients-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
  recipients = []
}
`, i)
}

func testAccAlertWithInverseScope(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-inverse-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type    = "fixed"
        id      = "cloud_provider"
        mode    = "is"
        inverse = true
        values  = ["google-cloud-platform"]
      }
    ]
  }
}
`, i)
}

func testAccAlertWithEvaluateForEach(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-evaluate-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval     = "month"
    value             = 100
    currency          = "USD"
    condition         = "value"
    operator          = "gt"
    evaluate_for_each = "fixed:service_description"
  }
}
`, i)
}

func testAccAlertPercentageChange(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-pct-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 50
    currency      = "USD"
    condition     = "percentage-change"
    operator      = "gt"
  }
}
`, i)
}

func testAccAlertWithOperator(i int, operator string) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-op-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "%s"
  }
}
`, i, operator)
}

func testAccAlertWithTimeInterval(i int, interval string) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-ti-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "%s"
    value         = 100
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}
`, i, interval)
}

func testAccAlertNoRecipients(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-no-recip-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}
`, i)
}

func testAccAlertAllConfigAttributes(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-full-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval     = "week"
    value             = 5000
    currency          = "EUR"
    condition         = "value"
    operator          = "gt"
    evaluate_for_each = "fixed:cloud_provider"
    scopes = [
      {
        type    = "fixed"
        id      = "cloud_provider"
        mode    = "is"
        inverse = false
        values  = ["amazon-web-services", "google-cloud-platform"]
      }
    ]
  }
}
`, i)
}

// TestAccAlert_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccAlert_Disappears(t *testing.T) {
	n := acctest.RandInt()
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource
			{
				Config: testAccAlert(n),
				Check: resource.ComposeTestCheckFunc(
					// Capture the resource ID for later deletion
					resource.TestCheckResourceAttrWith("doit_alert.this", "id", func(value string) error {
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
					resp, err := client.DeleteAlertWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete alert via API: %v", err)
					}
					// 200 or 204 = success, 404 = already deleted (both are OK)
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Unexpected status code when deleting alert: %d, body: %s",
							resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccAlert(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}

// TestAccAlert_WithExplicitRecipient verifies that the CI user email works as a recipient.
// This test checks if the API domain validation issue has been fixed.
func TestAccAlert_WithExplicitRecipient(t *testing.T) {
	n := acctest.RandInt()
	testUser := os.Getenv("TEST_USER")
	if testUser == "" {
		t.Skip("TEST_USER not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_alert" "test_recipient" {
  name = "test-explicit-recipient-%d"
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
  recipients = ["%s"]
}
`, n, testUser),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.test_recipient",
						tfjsonpath.New("recipients"),
						knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

// TestAccAlert_ScopesAliasTypes tests that using the "allocation_rule" alias type
// in alert scopes is handled correctly. Note: As of now, the alert API only accepts
// "attribution" (not the "allocation_rule" alias), so we test with "attribution".
// The normalization code in the provider ensures that if the API is updated to accept
// aliases in the future, or if the user imports a resource that was created with aliases,
// the state will remain consistent.
func TestAccAlert_ScopesAliasTypes(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertScopesAliasTypes(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify type and id are preserved in state
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type": knownvalue.StringExact("attribution"),
								"id":   knownvalue.StringExact("attribution"),
								"mode": knownvalue.StringExact("is"),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccAlertScopesAliasTypes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertScopesAliasTypes(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-alias-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "attribution"
        id     = "attribution"
        mode   = "is"
        values = ["%s"]
      }
    ]
  }
}
`, i, testAttribution())
}

// TestAccAlert_IncludeNull tests that the include_null property on alert scopes
// round-trips correctly without causing drift.
func TestAccAlert_IncludeNull(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithIncludeNull(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
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
				Config: testAccAlertWithIncludeNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertWithIncludeNull(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-include-null-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type         = "fixed"
        id           = "service_description"
        mode         = "is"
        include_null = true
        values       = ["Compute Engine"]
      }
    ]
  }
}
`, i)
}

// TestAccAlert_CaseInsensitive tests that the case_insensitive property on alert scopes
// round-trips correctly without causing drift. Uses mode="contains" with a lowercase
// value to exercise the case-insensitive matching path.
func TestAccAlert_CaseInsensitive(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithCaseInsensitive(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
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
				Config: testAccAlertWithCaseInsensitive(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertWithCaseInsensitive(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-case-insensitive-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type             = "fixed"
        id               = "cloud_provider"
        mode             = "contains"
        case_insensitive = true
        values           = ["aws"]
      }
    ]
  }
}
`, i)
}

// TestAccAlert_ScopeIncludeNull tests that a scope with include_null = true
// and a real value round-trips correctly without drift.
// This is the alert equivalent of TestAccReport_IncludeNull.
// It exercises the state-first includeNullVal logic: the API may not reliably
// echo include_null back, so the provider must preserve the user's value.
func TestAccAlert_ScopeIncludeNull(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertScopeIncludeNull(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
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
				Config: testAccAlertScopeIncludeNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertScopeIncludeNull(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-include-null-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type         = "fixed"
        id           = "service_description"
        mode         = "is"
        include_null = true
        values       = ["Compute Engine"]
      }
    ]
  }
}
`, i)
}

// TestAccAlert_IncludeNullOnlyNoValues tests that a scope with include_null = true
// and NO values is accepted by the API and round-trips without drift.
//
// PR #51575 (fix(analytics): allow include_null and empty values public-api) is
// deployed and the alert API accepts this configuration. This test verifies the
// full round-trip: the provider sends include_null=true with an empty values list,
// the API stores it, and the provider reads it back without drift.
//
// If this test fails with a provider inconsistency error, check that alert.go
// correctly maps a nil/empty API values list to an empty Terraform list (not null)
// when include_null=true is set in the scope.
func TestAccAlert_IncludeNullOnlyNoValues(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertIncludeNullOnlyNoValues(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
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
				Config: testAccAlertIncludeNullOnlyNoValues(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertIncludeNullOnlyNoValues(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-include-null-only-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type         = "fixed"
        id           = "service_description"
        mode         = "is"
        include_null = true
        values       = []
      }
    ]
  }
}
`, i)
}

// TestAccAlert_ScopeWithNAValue tests that a scope configured with the legacy
// NullFallback sentinel value (e.g. "[Service N/A]") round-trips correctly —
// i.e. after apply the state still contains "[Service N/A]" and no drift is
// reported on a subsequent plan.
//
// This is the alert equivalent of TestAccReport_FilterValuesMixedWithNA.
//
// With the sentinel-restoration logic in alert.go (mirroring isNAFallback +
// populateState in report.go), this test is expected to PASS and now serves
// as a regression guard to ensure "[Service N/A]" is preserved in state and
// does not produce drift.
//
// The failure mode (if the restoration logic regresses) is
// "Provider produced inconsistent result after apply": the provider sends
// "[Service N/A]" → the API strips it and sets include_null=true → on read the
// provider gets back include_null=true + values=[] → state becomes values=[],
// which mismatches the configured "[Service N/A]".
func TestAccAlert_ScopeWithNAValue(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertWithNAValue(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("fixed"),
								"id":     knownvalue.StringExact("service_description"),
								"mode":   knownvalue.StringExact("is"),
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("[Service N/A]")}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccAlertWithNAValue(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertWithNAValue(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-na-value-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "fixed"
        id     = "service_description"
        mode   = "is"
        values = ["[Service N/A]"]
      }
    ]
  }
}
`, i)
}

// TestAccAlert_ScopeWithMixedNAValue reproduces the scenario where a scope
// contains BOTH a real value AND a legacy "[... N/A]" sentinel, e.g.:
//
//	values = ["Compute Engine", "[Service N/A]"]
//
// The API strips the sentinel and returns values=["Compute Engine"] + include_null=true.
// Unlike the pure-NA case (where the API returns values=[] and the blunt all-NA
// fallback fires), here apiHasValues=true so the current fallback does NOT restore
// the sentinel, causing perpetual drift.
// This test is the alert equivalent of TestAccReport_FilterValuesMixedWithNA.
// The smarter include_null-based sentinel detection (mergeSentinelValues, called
// from alert.go's populateState) is now implemented in this PR. This test serves
// as a regression guard to ensure the mixed-sentinel restoration continues to work.
// See: https://doitintl.atlassian.net/browse/CMP-38116
func TestAccAlert_ScopeWithMixedNAValue(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with both a real value and a sentinel.
			// The provider must restore "[Service N/A]" after the API strips it,
			// so state contains both values as the user configured.
			{
				Config: testAccAlertWithMixedNAValue(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type": knownvalue.StringExact("fixed"),
								"id":   knownvalue.StringExact("service_description"),
								"mode": knownvalue.StringExact("is"),
								// Both the real value and the sentinel must be present in state.
								"values": knownvalue.ListExact([]knownvalue.Check{
									knownvalue.StringExact("Compute Engine"),
									knownvalue.StringExact("[Service N/A]"),
								}),
							}),
						}),
					),
				},
			},
			// Step 2: Verify no drift on re-apply — critical regression guard.
			{
				Config: testAccAlertWithMixedNAValue(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertWithMixedNAValue(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-mixed-na-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "fixed"
        id     = "service_description"
        mode   = "is"
        values = ["Compute Engine", "[Service N/A]"]
      }
    ]
  }
}
`, i)
}

// TestAccAlert_MinimalConfig tests creating an alert with only the absolute
// minimum required fields, omitting all Optional+Computed config scalars
// (condition, currency, operator, time_interval, evaluate_for_each, data_source).
// This exercises the overlay code's handling of Unknown values for every
// Optional+Computed field in the config object.
func TestAccAlert_MinimalConfig(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with minimal config.
			{
				Config: testAccAlertMinimalConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-alert-minimal-%d", n))),
					// time_interval defaults to "year" via schema default
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("time_interval"),
						knownvalue.StringExact("year")),
					// Scopes should be empty list (not null)
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("scopes"),
						knownvalue.ListSizeExact(0)),
					// Attributions should be empty list (not null)
					statecheck.ExpectKnownValue(
						"doit_alert.this",
						tfjsonpath.New("config").AtMapKey("attributions"),
						knownvalue.ListSizeExact(0)),
				},
			},
			// Step 2: Drift check.
			{
				Config: testAccAlertMinimalConfig(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertMinimalConfig(i int) string {
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-minimal-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    value    = 500
    operator = "gt"
  }
}
`, i)
}
