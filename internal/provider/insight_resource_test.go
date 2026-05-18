package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccInsightResource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccInsightResourceConfig(rName, "Basic Test Insight", "A basic test insight"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_insight.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// Required fields
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("key"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("Basic Test Insight")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("A basic test insight")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("cloud_provider"),
						knownvalue.StringExact("aws")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("categories"),
						knownvalue.ListSizeExact(1)),

					// Optional+Computed fields should be null when not set
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("detailed_description_mdx"),
						knownvalue.Null()),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("easy_win_description"),
						knownvalue.Null()),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("report_url"),
						knownvalue.Null()),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("cloud_flow_template_id"),
						knownvalue.Null()),

					// Computed-only: summary
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("summary"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("summary").AtMapKey("potential_daily_savings"),
						knownvalue.Float64Exact(0)),

					// Computed-only: last_status_change
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("last_status_change"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Drift check — re-apply same config, expect no changes
			{
				Config: testAccInsightResourceConfig(rName, "Basic Test Insight", "A basic test insight"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResource_AllFields tests creating an insight with every optional
// metadata field populated (detailed_description_mdx, easy_win_description,
// report_url, cloud_flow_template_id). Tags are Computed-only (read from
// customer_insights). The Basic test above covers the minimal required-only config.
func TestAccInsightResource_AllFields(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with all optional fields
			{
				Config: testAccInsightResourceAllFieldsConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Required fields
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("key"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("All Fields Test")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("Testing all optional metadata fields")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("cloud_provider"),
						knownvalue.StringExact("aws")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("categories"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("FinOps"),
						})),

					// All optional user-configurable fields
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("detailed_description_mdx"),
						knownvalue.StringExact("# Detailed\n\nThis is a **detailed** description.")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("easy_win_description"),
						knownvalue.StringExact("Resize the instance to save costs.")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("report_url"),
						knownvalue.StringExact(fmt.Sprintf("https://console.doit.com/customers/%s/analytics/reports/test-report", os.Getenv("TEST_CUSTOMER_ID")))), statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("tags"),
						knownvalue.ListSizeExact(0)),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("cloud_flow_template_id"),
						knownvalue.StringExact("tmpl-12345")),

					// Computed-only fields — verify they are populated
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("public-api")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("source_id"),
						knownvalue.StringExact("public-api")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("insight_key"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("display_status"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("last_updated"),
						knownvalue.NotNull()),

					// Computed-only: summary with all sub-fields
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("summary"),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"operational_risks":       knownvalue.Float64Exact(0),
							"performance_risks":       knownvalue.Float64Exact(0),
							"potential_daily_savings": knownvalue.Float64Exact(0),
							"reliability_risks":       knownvalue.Float64Exact(0),
							"security_risks":          knownvalue.Float64Exact(0),
							"sustainability_risks":    knownvalue.Float64Exact(0),
						})),

					// Computed-only: last_status_change
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("last_status_change"),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"last_changed_at": knownvalue.NotNull(),
							"user_id":         knownvalue.NotNull(),
						})),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccInsightResourceAllFieldsConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccInsightResource_Update(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccInsightResourceConfig(rName, "Original Title", "Original description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("Original Title")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("Original description")),
				},
			},
			// Step 2: Update title and description
			{
				Config: testAccInsightResourceConfig(rName, "Updated Title", "Updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("Updated Title")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("Updated description")),
				},
			},
			// Step 3: Drift check
			{
				Config: testAccInsightResourceConfig(rName, "Updated Title", "Updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResource_UpdateOptionalFields tests updating optional metadata fields:
// minimal config → all optional fields → back to minimal.
func TestAccInsightResource_UpdateOptionalFields(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with minimal config (required fields only)
			{
				Config: testAccInsightResourceConfig(rName, "Minimal Title", "Minimal description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("Minimal Title")),
				},
			},
			// Step 2: Update to all optional fields populated
			{
				Config: testAccInsightResourceAllFieldsConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("All Fields Test")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("detailed_description_mdx"),
						knownvalue.StringExact("# Detailed\n\nThis is a **detailed** description.")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("easy_win_description"),
						knownvalue.StringExact("Resize the instance to save costs.")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("report_url"),
						knownvalue.StringExact(fmt.Sprintf("https://console.doit.com/customers/%s/analytics/reports/test-report", os.Getenv("TEST_CUSTOMER_ID")))), statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("cloud_flow_template_id"),
						knownvalue.StringExact("tmpl-12345")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("tags"),
						knownvalue.ListSizeExact(0)),
				},
			},
			// Step 3: Drift check after adding optional fields
			{
				Config: testAccInsightResourceAllFieldsConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 4: Update back to minimal (remove optional fields + change title)
			{
				Config: testAccInsightResourceConfig(rName, "Back to Minimal", "Testing all optional metadata fields"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			// Step 5: Drift check after removing optional fields
			{
				Config: testAccInsightResourceConfig(rName, "Back to Minimal", "Testing all optional metadata fields"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccInsightResource_ImportState(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccInsightResourceConfig(rName, "Import Test Insight", "Test import functionality"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("key"),
						knownvalue.StringExact(rName)),
				},
			},
			// Step 2: Import
			{
				ResourceName:                         "doit_insight.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("public-api/%s", rName),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "key",
				// timeouts are client-only
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			// Step 3: Drift check — re-apply config after import, expect no changes
			{
				Config: testAccInsightResourceConfig(rName, "Import Test Insight", "Test import functionality"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// testAccInsightResourceConfig generates a minimal insight resource config (metadata only).
// Resource results are now managed by the separate doit_insight_resource_results resource.
func testAccInsightResourceConfig(key, title, description string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = %[2]q
  short_description = %[3]q
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}
`, key, title, description)
}

// testAccInsightResourceAllFieldsConfig generates a config with every optional metadata field set.
func testAccInsightResourceAllFieldsConfig(key string) string {
	customerID := os.Getenv("TEST_CUSTOMER_ID")
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "All Fields Test"
  short_description = "Testing all optional metadata fields"
  cloud_provider    = "aws"
  categories        = ["FinOps"]

  detailed_description_mdx = "# Detailed\n\nThis is a **detailed** description."
  easy_win_description     = "Resize the instance to save costs."
  report_url               = "https://console.doit.com/customers/%[2]s/analytics/reports/test-report"
  cloud_flow_template_id   = "tmpl-12345"
}
`, key, customerID)
}

// TestAccInsightResource_Status tests creating an insight with an explicit status.
func TestAccInsightResource_Status(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with status = "acknowledged"
			{
				Config: testAccInsightResourceWithStatusConfig(rName, "acknowledged"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("status"),
						knownvalue.StringExact("acknowledged")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("display_status"),
						knownvalue.StringExact("acknowledged")),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccInsightResourceWithStatusConfig(rName, "acknowledged"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResource_StatusUpdate tests changing the status of an insight.
func TestAccInsightResource_StatusUpdate(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with default status (omitted — API defaults to "dismissed")
			{
				Config: testAccInsightResourceConfig(rName, "Status Update Test", "Testing status transitions"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("display_status"),
						knownvalue.StringExact("dismissed")),
				},
			},
			// Step 2: Update to "acknowledged"
			{
				Config: testAccInsightResourceWithStatusConfig(rName, "acknowledged"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("status"),
						knownvalue.StringExact("acknowledged")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("display_status"),
						knownvalue.StringExact("acknowledged")),
				},
			},
			// Step 3: Update to "in progress"
			{
				Config: testAccInsightResourceWithStatusConfig(rName, "in progress"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("status"),
						knownvalue.StringExact("in progress")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("display_status"),
						knownvalue.StringExact("in progress")),
				},
			},
			// Step 4: Drift check
			{
				Config: testAccInsightResourceWithStatusConfig(rName, "in progress"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResource_Dismissed tests creating an insight with "dismissed" status
// and the required dismissal_details.
func TestAccInsightResource_Dismissed(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with status = "dismissed" + dismissal_details
			{
				Config: testAccInsightResourceDismissedConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("status"),
						knownvalue.StringExact("dismissed")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("display_status"),
						knownvalue.StringExact("dismissed")),
					statecheck.ExpectKnownValue(
						"doit_insight.test",
						tfjsonpath.New("dismissal_details"),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"reason":  knownvalue.StringExact("not relevant"),
							"comment": knownvalue.StringExact("Managed by Terraform test"),
						})),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccInsightResourceDismissedConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightResourceWithStatusConfig(key, status string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Status Test"
  short_description = "Testing status transitions"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
  status            = %[2]q
}
`, key, status)
}

func testAccInsightResourceDismissedConfig(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Dismissed Test"
  short_description = "Testing dismissal with details"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
  status            = "dismissed"

  dismissal_details = {
    reason  = "not relevant"
    comment = "Managed by Terraform test"
  }
}
`, key)
}
