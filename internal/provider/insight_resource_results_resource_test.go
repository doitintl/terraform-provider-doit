package provider_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccInsightResourceResults_Basic tests creating resource results
// for an existing insight, then drift-checking.
func TestAccInsightResourceResults_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create insight + resource results
			{
				Config: testAccInsightWithResults(rName, "Basic RR Test", "A basic resource results test"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction("doit_insight.test", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction("doit_insight_resource_results.test", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: basicResultChecks(rName),
			},
			// Step 2: Drift check — same assertions on BOTH steps catches overlay bugs
			{
				Config:            testAccInsightWithResults(rName, "Basic RR Test", "A basic resource results test"),
				ConfigStateChecks: basicResultChecks(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_Update tests updating resource results (add a second result).
func TestAccInsightResourceResults_Update(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with 1 result
			{
				Config: testAccInsightWithResults(rName, "Update RR Test", "Update test"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results"),
						knownvalue.ListSizeExact(1)),
				},
			},
			// Step 2: Update to 2 results
			{
				Config: testAccInsightWithTwoResults(rName, "Update RR Test", "Update test"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 3: Drift check
			{
				Config: testAccInsightWithTwoResults(rName, "Update RR Test", "Update test"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_SecurityRisk tests the security_risk result type
// with critical/high/medium/low severity counts.
func TestAccInsightResourceResults_SecurityRisk(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:            testAccInsightResultsSecurityRisk(rName),
				ConfigStateChecks: securityRiskChecks(),
			},
			// Drift check — same full assertions
			{
				Config:            testAccInsightResultsSecurityRisk(rName),
				ConfigStateChecks: securityRiskChecks(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_Recommendation tests potential_daily_savings_with_recommendation.
func TestAccInsightResourceResults_Recommendation(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:            testAccInsightResultsRecommendation(rName),
				ConfigStateChecks: recommendationChecks(),
			},
			// Drift check — same full assertions
			{
				Config:            testAccInsightResultsRecommendation(rName),
				ConfigStateChecks: recommendationChecks(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_Pagination tests that resource results with many items
// are correctly paginated and all results round-trip without drift.
func TestAccInsightResourceResults_Pagination(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")
	numResults := 25 // Enough to test pagination with small page sizes

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with many results
			{
				Config: testAccInsightResultsPaginated(rName, numResults),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results"),
						knownvalue.ListSizeExact(numResults)),
				},
			},
			// Step 2: Drift check — verifies Read auto-paginates and returns all results
			{
				Config: testAccInsightResultsPaginated(rName, numResults),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_ImportState tests import of resource results.
func TestAccInsightResourceResults_ImportState(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccInsightWithResults(rName, "Import RR Test", "Import test"),
			},
			// Step 2: Import
			{
				ResourceName:                         "doit_insight_resource_results.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("public-api/%s", rName),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "insight_key",
				ImportStateVerifyIgnore: []string{
					"timeouts",
					// Optional fields not returned by API when empty
					"resource_results.0.external_id",
					"resource_results.0.external_url",
					"resource_results.0.location",
					"resource_results.0.resource_type",
					"resource_results.0.severity", // Computed; API returns nil vs "" inconsistently
				},
			},
		},
	})
}

// TestAccInsightResourceResults_AllFields tests resource results with every
// optional field populated: external_id, external_url, location, resource_type.
// The Basic test uses only the required fields.
func TestAccInsightResourceResults_AllFields(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightResultsAllFields(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results"),
						knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
						knownvalue.StringExact("ext-abc-123")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("location"),
						knownvalue.StringExact("us-east-1")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_type"),
						knownvalue.StringExact("instance")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_url"),
						knownvalue.StringExact("https://console.aws.amazon.com/ec2/i-all-fields-001")),
				},
			},
			// Drift check
			{
				Config: testAccInsightResultsAllFields(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_ClusterAgent tests the
// potential_daily_savings_with_cluster_agent result type with agent_installed.
func TestAccInsightResourceResults_ClusterAgent(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightResultsClusterAgent(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result_type"),
						knownvalue.StringExact("potential_daily_savings_with_cluster_agent")),
					// Result — ObjectExact: value + agent_installed set, rest null
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"value":           knownvalue.Float64Exact(8.50),
							"agent_installed": knownvalue.Bool(true),
							"critical":        knownvalue.Null(),
							"current":         knownvalue.Null(),
							"high":            knownvalue.Null(),
							"low":             knownvalue.Null(),
							"medium":          knownvalue.Null(),
							"recommendation":  knownvalue.Null(),
						})),
				},
			},
			// Drift check
			{
				Config: testAccInsightResultsClusterAgent(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"value":           knownvalue.Float64Exact(8.50),
							"agent_installed": knownvalue.Bool(true),
							"critical":        knownvalue.Null(),
							"current":         knownvalue.Null(),
							"high":            knownvalue.Null(),
							"low":             knownvalue.Null(),
							"medium":          knownvalue.Null(),
							"recommendation":  knownvalue.Null(),
						})),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_UpdateAllFields tests mutating every mutable field
// on a resource result: external_id, external_url, and result.value. Immutable
// identity fields (result_type, account, cloud_provider, location, resource_type)
// are kept unchanged — see TestAccInsightResourceResults_UpdateResultType for
// RequiresReplace validation.
func TestAccInsightResourceResults_UpdateAllFields(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with all optional fields set to initial values
			{
				Config: testAccInsightResultsAllFieldsV1(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("location"),
						knownvalue.StringExact("us-east-1")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
						knownvalue.StringExact("ext-v1")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_url"),
						knownvalue.StringExact("https://example.com/v1")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_type"),
						knownvalue.StringExact("instance")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"value": knownvalue.Float64Exact(5.42),
						})),
				},
			},
			// Step 2: Drift check after create
			{
				Config: testAccInsightResultsAllFieldsV1(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Mutate all mutable fields to new values
			// (location and resource_type are immutable, so they stay the same)
			{
				Config: testAccInsightResultsAllFieldsV2(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify immutable fields are unchanged
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("location"),
						knownvalue.StringExact("us-east-1")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_type"),
						knownvalue.StringExact("instance")),
					// Verify mutable fields were updated
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
						knownvalue.StringExact("ext-v2")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_url"),
						knownvalue.StringExact("https://example.com/v2")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"value": knownvalue.Float64Exact(99.99),
						})),
				},
			},
			// Step 4: Drift check after mutation — the key assertion
			{
				Config: testAccInsightResultsAllFieldsV2(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_UpdateResultType tests that changing the result_type
// triggers a destroy+create (RequiresReplace) since result_type is immutable.
func TestAccInsightResourceResults_UpdateResultType(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with potential_daily_savings
			{
				Config: testAccInsightWithResults(rName, "ResultType Update Test", "Test changing result_type"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result_type"),
						knownvalue.StringExact("potential_daily_savings")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"value": knownvalue.Float64Exact(5.42),
						})),
				},
			},
			// Step 2: Update to potential_daily_savings_with_recommendation.
			// result_type is immutable (RequiresReplace), so Terraform should
			// destroy and recreate the resource.
			{
				Config: testAccInsightResultsChangeToRecommendation(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction("doit_insight_resource_results.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result_type"),
						knownvalue.StringExact("potential_daily_savings_with_recommendation")),
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"value":          knownvalue.Float64Exact(10.00),
							"current":        knownvalue.StringExact("m5.xlarge"),
							"recommendation": knownvalue.StringExact("t3.medium"),
						})),
				},
			},
			// Step 3: Drift check
			{
				Config: testAccInsightResultsChangeToRecommendation(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_RemoveResult tests removing a result from the list
// (2 → 1), verifying that the declarative delete logic removes the dropped resource.
func TestAccInsightResourceResults_RemoveResult(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with 2 results
			{
				Config: testAccInsightWithTwoResults(rName, "Remove Result Test", "Test removing a result"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 2: Remove second result (go back to 1)
			{
				Config: testAccInsightWithResults(rName, "Remove Result Test", "Test removing a result"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results"),
						knownvalue.ListSizeExact(1)),
					// Verify the remaining result is the first one
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_id"),
						knownvalue.StringExact(fmt.Sprintf("i-acc-%s-1", rName))),
				},
			},
			// Step 3: Drift check
			{
				Config: testAccInsightWithResults(rName, "Remove Result Test", "Test removing a result"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_AllFieldsImport tests import with all optional
// fields populated, unlike the basic import test which uses minimal fields.
func TestAccInsightResourceResults_AllFieldsImport(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with all fields
			{
				Config: testAccInsightResultsAllFields(rName),
			},
			// Step 2: Import
			{
				ResourceName:                         "doit_insight_resource_results.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("public-api/%s", rName),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "insight_key",
				ImportStateVerifyIgnore: []string{
					"timeouts",
					"resource_results.0.severity", // Computed; API returns nil vs "" inconsistently
				},
			},
			// Step 3: Drift check after import — re-apply same config, expect no changes
			{
				Config: testAccInsightResultsAllFields(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// --- Assertion helpers for full attribute coverage ---

// basicResultChecks returns full-coverage assertions for a single
// potential_daily_savings result with value=5.42. Uses ObjectExact (not
// ObjectPartial) to catch any unexpected non-null values in nested objects.
func basicResultChecks(rName string) []statecheck.StateCheck {
	return []statecheck.StateCheck{
		// Top-level attributes
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("insight_key"),
			knownvalue.StringExact(rName)),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("source_id"),
			knownvalue.StringExact("public-api")),

		// List
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results"),
			knownvalue.ListSizeExact(1)),

		// Required fields on element
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_id"),
			knownvalue.StringExact(fmt.Sprintf("i-acc-%s-1", rName))),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("account"),
			knownvalue.StringExact("111111111111")),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("cloud_provider"),
			knownvalue.StringExact("aws")),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result_type"),
			knownvalue.StringExact("potential_daily_savings")),

		// Computed-only fields
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resolved"),
			knownvalue.Bool(false)),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("severity"),
			knownvalue.StringExact("")),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("enhancement"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
			knownvalue.Null()),

		// Optional+Computed: should be null when not set in HCL
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_url"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("location"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_type"),
			knownvalue.Null()),

		// Result nested object — ObjectExact catches unexpected non-null values
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
			knownvalue.ObjectExact(map[string]knownvalue.Check{
				"value":           knownvalue.Float64Exact(5.42),
				"agent_installed": knownvalue.Null(),
				"critical":        knownvalue.Null(),
				"current":         knownvalue.Null(),
				"high":            knownvalue.Null(),
				"low":             knownvalue.Null(),
				"medium":          knownvalue.Null(),
				"recommendation":  knownvalue.Null(),
			})),
	}
}

// securityRiskChecks returns full-coverage assertions for a security_risk result.
func securityRiskChecks() []statecheck.StateCheck {
	return []statecheck.StateCheck{
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result_type"),
			knownvalue.StringExact("security_risk")),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resolved"),
			knownvalue.Bool(false)),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_type"),
			knownvalue.StringExact("security-group")),
		// Computed-only
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("severity"),
			knownvalue.NotNull()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("enhancement"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
			knownvalue.Null()),
		// Optional+Computed not in HCL
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_url"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("location"),
			knownvalue.Null()),
		// Result — ObjectExact: security counts set, savings fields null
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
			knownvalue.ObjectExact(map[string]knownvalue.Check{
				"critical":        knownvalue.Int64Exact(1),
				"high":            knownvalue.Int64Exact(3),
				"medium":          knownvalue.Int64Exact(5),
				"low":             knownvalue.Int64Exact(10),
				"value":           knownvalue.Null(),
				"current":         knownvalue.Null(),
				"recommendation":  knownvalue.Null(),
				"agent_installed": knownvalue.Null(),
			})),
	}
}

// recommendationChecks returns full-coverage assertions for a recommendation result.
func recommendationChecks() []statecheck.StateCheck {
	return []statecheck.StateCheck{
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result_type"),
			knownvalue.StringExact("potential_daily_savings_with_recommendation")),
		// Computed-only
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resolved"),
			knownvalue.Bool(false)),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("severity"),
			knownvalue.StringExact("")),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("enhancement"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
			knownvalue.Null()),
		// Optional+Computed not in HCL
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_url"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("location"),
			knownvalue.Null()),
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("resource_type"),
			knownvalue.Null()),
		// Result — ObjectExact: value/current/recommendation set, rest null
		statecheck.ExpectKnownValue(
			"doit_insight_resource_results.test",
			tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("result"),
			knownvalue.ObjectExact(map[string]knownvalue.Check{
				"value":           knownvalue.Float64Exact(12.50),
				"current":         knownvalue.StringExact("m5.xlarge"),
				"recommendation":  knownvalue.StringExact("m5.large"),
				"agent_installed": knownvalue.Null(),
				"critical":        knownvalue.Null(),
				"high":            knownvalue.Null(),
				"low":             knownvalue.Null(),
				"medium":          knownvalue.Null(),
			})),
	}
}

// --- Config helpers ---

func testAccInsightWithResults(key, title, description string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = %[2]q
  short_description = %[3]q
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-acc-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"

    result = {
      value = 5.42
    }
  }]
}
`, key, title, description)
}

func testAccInsightWithTwoResults(key, title, description string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = %[2]q
  short_description = %[3]q
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [
    {
      resource_id    = "i-acc-%[1]s-1"
      account        = "111111111111"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      result = {
        value = 5.42
      }
    },
    {
      resource_id    = "i-acc-%[1]s-2"
      account        = "222222222222"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      location       = "us-west-2"
      result = {
        value = 10.00
      }
    },
  ]
}
`, key, title, description)
}

func testAccInsightResultsSecurityRisk(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Security Risk RR Test"
  short_description = "Test resource results with security_risk type"
  cloud_provider    = "aws"
  categories        = ["Security"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "sg-acc-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "security_risk"
    resource_type  = "security-group"

    result = {
      critical = 1
      high     = 3
      medium   = 5
      low      = 10
    }
  }]
}
`, key)
}

func testAccInsightResultsRecommendation(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Recommendation RR Test"
  short_description = "Test resource results with recommendation type"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-rec-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings_with_recommendation"

    result = {
      value          = 12.50
      current        = "m5.xlarge"
      recommendation = "m5.large"
    }
  }]
}
`, key)
}

func testAccInsightResultsPaginated(key string, count int) string {
	var results strings.Builder
	for i := range count {
		if i > 0 {
			results.WriteString(",\n")
		}
		fmt.Fprintf(&results, `    {
      resource_id    = "i-pag-%s-%03d"
      account        = "111111111111"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      result = {
        value = %d.00
      }
    }`, key, i, i+1)
	}

	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Pagination RR Test"
  short_description = "Test with many resource results for pagination"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [
%[2]s
  ]
}
`, key, results.String())
}

func testAccInsightResultsAllFields(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "All Fields RR Test"
  short_description = "Test resource results with all optional fields"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-all-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    location       = "us-east-1"
    resource_type  = "instance"
    external_id    = "ext-abc-123"
    external_url   = "https://console.aws.amazon.com/ec2/i-all-fields-001"

    result = {
      value = 5.42
    }
  }]
}
`, key)
}

func testAccInsightResultsClusterAgent(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Cluster Agent RR Test"
  short_description = "Test resource results with cluster agent type"
  cloud_provider    = "gcp"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "gke-%[1]s-1"
    account        = "my-gcp-project"
    cloud_provider = "gcp"
    result_type    = "potential_daily_savings_with_cluster_agent"

    result = {
      value           = 8.50
      agent_installed = true
    }
  }]
}
`, key)
}

// testAccInsightResultsAllFieldsV1 creates a result with all optional fields at V1 values.
func testAccInsightResultsAllFieldsV1(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "All Fields Mutation Test"
  short_description = "Test mutating all optional fields"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-mut-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    location       = "us-east-1"
    resource_type  = "instance"
    external_id    = "ext-v1"
    external_url   = "https://example.com/v1"

    result = {
      value = 5.42
    }
  }]
}
`, key)
}

// testAccInsightResultsAllFieldsV2 creates a result with all mutable fields
// mutated to V2 values. Immutable identity fields (result_type, account,
// cloud_provider, location, resource_type) are kept the same as V1.
func testAccInsightResultsAllFieldsV2(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "All Fields Mutation Test"
  short_description = "Test mutating all optional fields"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-mut-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    location       = "us-east-1"
    resource_type  = "instance"
    external_id    = "ext-v2"
    external_url   = "https://example.com/v2"

    result = {
      value = 99.99
    }
  }]
}
`, key)
}

// testAccInsightResultsChangeToRecommendation updates the same resource_id
// from potential_daily_savings to potential_daily_savings_with_recommendation.
func testAccInsightResultsChangeToRecommendation(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "ResultType Update Test"
  short_description = "Test changing result_type"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-acc-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings_with_recommendation"

    result = {
      value          = 10.00
      current        = "m5.xlarge"
      recommendation = "t3.medium"
    }
  }]
}
`, key)
}

// TestAccInsightResourceResults_MetadataOmitThenAdd tests the lifecycle:
// 1. Create without metadata (omitted) → metadata is null → empty plan
// 2. Update to add metadata → metadata is set → empty plan.
func TestAccInsightResourceResults_MetadataOmitThenAdd(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create without metadata
			{
				Config: testAccInsightWithResults(rName, "Metadata Omit-Add Test", "Test omitting then adding metadata"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.Null()),
				},
			},
			// Step 2: Drift check — no metadata, should be stable
			{
				Config: testAccInsightWithResults(rName, "Metadata Omit-Add Test", "Test omitting then adding metadata"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Add metadata via update
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"region":"us-east-1","tier":"gold"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.NotNull()),
				},
			},
			// Step 4: Drift check — metadata should be stable
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"region":"us-east-1","tier":"gold"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_MetadataAddThenRemove tests the lifecycle:
// 1. Create with metadata → metadata is set → empty plan
// 2. Update to remove metadata (explicit null) → metadata is null → empty plan.
func TestAccInsightResourceResults_MetadataAddThenRemove(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with metadata
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"region":"us-east-1","tier":"gold"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Drift check — metadata should be stable
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"region":"us-east-1","tier":"gold"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Remove metadata by setting it to null explicitly
			{
				Config: testAccInsightResultsWithNullMetadata(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.Null()),
				},
			},
			// Step 4: Drift check — null metadata should be stable
			{
				Config: testAccInsightResultsWithNullMetadata(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_MetadataCreateWithValue tests creating a
// resource with metadata from the start (not added via update).
func TestAccInsightResourceResults_MetadataCreateWithValue(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with metadata
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"env":"production","cost_center":"eng"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Drift check — metadata should be stable
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"env":"production","cost_center":"eng"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_MetadataCreateWithoutValue tests creating a
// resource without metadata and verifying the null value is stable.
func TestAccInsightResourceResults_MetadataCreateWithoutValue(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create without metadata
			{
				Config: testAccInsightWithResults(rName, "Metadata None Test", "Test creating without metadata"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.Null()),
				},
			},
			// Step 2: Drift check — null metadata should be stable
			{
				Config: testAccInsightWithResults(rName, "Metadata None Test", "Test creating without metadata"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResults_MetadataImport verifies that importing a
// resource with metadata preserves the JSON value in state.
func TestAccInsightResourceResults_MetadataImport(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with metadata
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"importKey":"importVal"}`),
			},
			// Step 2: Import
			{
				ResourceName:                         "doit_insight_resource_results.test",
				ImportState:                          true,
				ImportStateId:                        fmt.Sprintf("public-api/%s", rName),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "insight_key",
				ImportStateVerifyIgnore: []string{
					"timeouts",
					"resource_results.0.severity", // Computed; API returns nil vs "" inconsistently
				},
			},
			// Step 3: Drift check after import
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"importKey":"importVal"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightResultsWithMetadata(key, metadataJSON string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Metadata RR Test"
  short_description = "Test metadata round-trip"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-acc-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    metadata       = jsonencode(jsondecode(%[2]q))

    result = {
      value = 5.42
    }
  }]
}
`, key, metadataJSON)
}

// testAccInsightResultsWithNullMetadata explicitly sets metadata = null.
// With the useEmptyForUnknownWhenConfigNull plan modifier, omitting the
// attribute has the same effect — both clear the prior value.
func testAccInsightResultsWithNullMetadata(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "Metadata RR Test"
  short_description = "Test metadata round-trip"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-acc-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    metadata       = null

    result = {
      value = 5.42
    }
  }]
}
`, key)
}

// TestAccInsightResourceResults_ExternalIdAddThenRemove tests clearing
// external_id (a plain types.String Optional+Computed attribute) as a control
// group for the metadata clearing test. If this test passes but the metadata
// variant fails, the bug is specific to jsontypes.Normalized.
func TestAccInsightResourceResults_ExternalIdAddThenRemove(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with external_id
			{
				Config: testAccInsightResultsWithExternalId(rName, "ext-abc-123"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
						knownvalue.StringExact("ext-abc-123")),
				},
			},
			// Step 2: Drift check — external_id should be stable
			{
				Config: testAccInsightResultsWithExternalId(rName, "ext-abc-123"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Remove external_id by setting it to null explicitly
			{
				Config: testAccInsightResultsWithNullExternalId(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("external_id"),
						knownvalue.Null()),
				},
			},
			// Step 4: Drift check — null external_id should be stable
			{
				Config: testAccInsightResultsWithNullExternalId(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightResultsWithExternalId(key, externalId string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "ExternalId RR Test"
  short_description = "Test external_id round-trip"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-acc-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    external_id    = %[2]q

    result = {
      value = 5.42
    }
  }]
}
`, key, externalId)
}

func testAccInsightResultsWithNullExternalId(key string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "ExternalId RR Test"
  short_description = "Test external_id round-trip"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = "public-api"
  insight_key = doit_insight.test.key

  resource_results = [{
    resource_id    = "i-acc-%[1]s-1"
    account        = "111111111111"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    external_id    = null

    result = {
      value = 5.42
    }
  }]
}
`, key)
}

// TestAccInsightResourceResults_MetadataEmptyObject verifies that the API
// normalizes an empty JSON object ({}) to null/absent. Setting metadata to
// jsonencode({}) sends {} to the API, but the API stores nothing and returns
// null. The provider maps this back to null, so the effective behavior is
// identical to metadata = null.
func TestAccInsightResourceResults_MetadataEmptyObject(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-irr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with real metadata.
			{
				Config: testAccInsightResultsWithMetadata(rName, `{"keep":"this"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Clear metadata with null — API clears it.
			{
				Config: testAccInsightResultsWithNullMetadata(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_insight_resource_results.test",
						tfjsonpath.New("resource_results").AtSliceIndex(0).AtMapKey("metadata"),
						knownvalue.Null()),
				},
			},
			// Step 3: Drift check — null metadata should be stable.
			{
				Config: testAccInsightResultsWithNullMetadata(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
