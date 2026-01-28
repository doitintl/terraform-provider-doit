package provider_test

import (
	"fmt"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAlert(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
		},
	})
}

func TestAccAlert_Import(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
		},
	})
}

func TestAccAlert_WithScopes(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
		},
	})
}

// TestAccAlert_WithEmptyScopes tests that explicitly setting scopes = [] works correctly.
// This verifies the fix for state inconsistency between empty list and null.
func TestAccAlert_WithEmptyScopes(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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

// TestAccAlert_WithInverseScope tests scope exclusion filters.
func TestAccAlert_WithInverseScope(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
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
  recipients = ["%s"]
}
`, i, testUser())
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
  recipients = ["%s"]
}
`, i, testUser())
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
  recipients = ["%s"]
}
`, i, testUser())
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
  recipients = ["%s"]
}
`, i, testUser())
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
  recipients = ["%s"]
}
`, i, operator, testUser())
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
  recipients = ["%s"]
}
`, i, interval, testUser())
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
  recipients = ["%s"]
}
`, i, testUser())
}
