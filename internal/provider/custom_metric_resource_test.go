package provider_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testCustomMetricAddr = "doit_custom_metric.this"

// customMetricVariable is one entry of variables in a test configuration.
type customMetricVariable struct {
	metricType string
	value      string
	allocation string // "a" or "b": the test allocation the variable measures
}

// testAccCustomMetricConfig returns two single allocations and a custom metric
// with the given attributes. extra holds optional attribute lines
// (description, unit_type, decimals) and may be empty.
func testAccCustomMetricConfig(name, formula string, variables []customMetricVariable, extra string) string {
	var vars strings.Builder
	for _, v := range variables {
		fmt.Fprintf(&vars, `    {
      metric        = { type = %q, value = %q }
      allocation_id = doit_allocation.%s.id
    },
`, v.metricType, v.value, v.allocation)
	}

	return fmt.Sprintf(`
resource "doit_allocation" "a" {
  name        = "%[1]s-a"
  description = "custom metric test allocation"
  rule = {
    formula = "A"
    components = [
      { key = "country", mode = "is", type = "fixed", values = ["JP"] },
    ]
  }
}

resource "doit_allocation" "b" {
  name        = "%[1]s-b"
  description = "custom metric test allocation"
  rule = {
    formula = "A"
    components = [
      { key = "country", mode = "is", type = "fixed", values = ["DE"] },
    ]
  }
}

resource "doit_custom_metric" "this" {
  name    = %[1]q
  formula = %[2]q
  variables = [
%[3]s  ]
%[4]s
}
`, name, formula, vars.String(), extra)
}

var (
	testCustomMetricCostA     = customMetricVariable{metricType: "basic", value: "cost", allocation: "a"}
	testCustomMetricUsageB    = customMetricVariable{metricType: "basic", value: "usage", allocation: "b"}
	testCustomMetricExtendedB = customMetricVariable{metricType: "extended", value: "amortized_cost", allocation: "b"}
	testCustomMetricSavingsA  = customMetricVariable{metricType: "basic", value: "savings", allocation: "a"}
)

// knownVariable checks one variables element: its letter, metric and the
// allocation it measures.
func knownVariable(letter, metricType, value string) knownvalue.Check {
	return knownvalue.ObjectPartial(map[string]knownvalue.Check{
		"letter": knownvalue.StringExact(letter),
		"metric": knownvalue.ObjectExact(map[string]knownvalue.Check{
			"type":  knownvalue.StringExact(metricType),
			"value": knownvalue.StringExact(value),
		}),
		"allocation_id": knownvalue.NotNull(),
	})
}

func expectEmptyPlanStep(config string) resource.TestStep {
	return resource.TestStep{
		Config: config,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{
				plancheck.ExpectEmptyPlan(),
			},
		},
	}
}

// TestAccCustomMetric_Basic creates a metric with every attribute set, then
// reorders, adds and removes variables. The letters follow the list order and
// are known at plan time.
func TestAccCustomMetric_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-cm")

	create := testAccCustomMetricConfig(rName, "A / B",
		[]customMetricVariable{testCustomMetricCostA, testCustomMetricExtendedB},
		`  description = "Cost per amortized cost"
  unit_type   = "percentage"
  decimals    = { min = 0, max = 2 }`)
	reordered := testAccCustomMetricConfig(rName+"-upd", "B / A",
		[]customMetricVariable{testCustomMetricExtendedB, testCustomMetricCostA},
		`  description = "Updated"
  unit_type   = "currency"
  decimals    = { min = 1, max = 3 }`)
	added := testAccCustomMetricConfig(rName+"-upd", "(B - C) / A",
		[]customMetricVariable{testCustomMetricExtendedB, testCustomMetricCostA, testCustomMetricSavingsA},
		`  description = "Updated"
  unit_type   = "currency"
  decimals    = { min = 1, max = 3 }`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: create,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testCustomMetricAddr, plancheck.ResourceActionCreate),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables").AtSliceIndex(0).AtMapKey("letter"), knownvalue.StringExact("A")),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables").AtSliceIndex(1).AtMapKey("letter"), knownvalue.StringExact("B")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("description"), knownvalue.StringExact("Cost per amortized cost")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("formula"), knownvalue.StringExact("A / B")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("unit_type"), knownvalue.StringExact("percentage")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"min": knownvalue.Int64Exact(0),
						"max": knownvalue.Int64Exact(2),
					})),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables"), knownvalue.ListExact([]knownvalue.Check{
						knownVariable("A", "basic", "cost"),
						knownVariable("B", "extended", "amortized_cost"),
					})),
					statecheck.CompareValuePairs(
						testCustomMetricAddr, tfjsonpath.New("variables").AtSliceIndex(0).AtMapKey("allocation_id"),
						"doit_allocation.a", tfjsonpath.New("id"),
						compare.ValuesSame(),
					),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("etag"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("owner"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("create_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("update_time"), knownvalue.NotNull()),
				},
				Check: testAccCheckCustomMetricEtagCurrent(t),
			},
			// Step 2: Drift check
			expectEmptyPlanStep(create),
			// Step 3: Rename, reorder the variables and change every optional attribute
			{
				Config: reordered,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testCustomMetricAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables"), knownvalue.ListExact([]knownvalue.Check{
							knownVariable("A", "extended", "amortized_cost"),
							knownVariable("B", "basic", "cost"),
						})),
						plancheck.ExpectUnknownValue(testCustomMetricAddr, tfjsonpath.New("etag")),
						plancheck.ExpectUnknownValue(testCustomMetricAddr, tfjsonpath.New("update_time")),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("owner"), knownvalue.NotNull()),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("create_time"), knownvalue.NotNull()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-upd")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("formula"), knownvalue.StringExact("B / A")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("unit_type"), knownvalue.StringExact("currency")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"min": knownvalue.Int64Exact(1),
						"max": knownvalue.Int64Exact(3),
					})),
					statecheck.CompareValuePairs(
						testCustomMetricAddr, tfjsonpath.New("variables").AtSliceIndex(1).AtMapKey("allocation_id"),
						"doit_allocation.a", tfjsonpath.New("id"),
						compare.ValuesSame(),
					),
				},
				Check: testAccCheckCustomMetricEtagCurrent(t),
			},
			// Step 4: Drift check
			expectEmptyPlanStep(reordered),
			// Step 5: Add a third variable
			{
				Config: added,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testCustomMetricAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables").AtSliceIndex(2).AtMapKey("letter"), knownvalue.StringExact("C")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables"), knownvalue.ListExact([]knownvalue.Check{
						knownVariable("A", "extended", "amortized_cost"),
						knownVariable("B", "basic", "cost"),
						knownVariable("C", "basic", "savings"),
					})),
				},
			},
			// Step 6: Drift check
			expectEmptyPlanStep(added),
			// Step 7: Remove it again
			{
				Config: reordered,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testCustomMetricAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables"), knownvalue.ListExact([]knownvalue.Check{
						knownVariable("A", "extended", "amortized_cost"),
						knownVariable("B", "basic", "cost"),
					})),
				},
			},
			// Step 8: Drift check
			expectEmptyPlanStep(reordered),
		},
	})
}

// TestAccCustomMetric_OmittedOptional omits description, unit_type and
// decimals: they resolve to "", "number" and null without drift.
func TestAccCustomMetric_OmittedOptional(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-cm-omit")
	config := testAccCustomMetricConfig(rName, "A", []customMetricVariable{testCustomMetricCostA}, "")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("description"), knownvalue.StringExact("")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("unit_type"), knownvalue.StringExact("number")),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("variables"), knownvalue.ListExact([]knownvalue.Check{
						knownVariable("A", "basic", "cost"),
					})),
				},
			},
			expectEmptyPlanStep(config),
		},
	})
}

// TestAccCustomMetric_DecimalsLifecycle sets decimals, removes it on its own,
// then sets it again and removes it in the same step as a name change.
func TestAccCustomMetric_DecimalsLifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-cm-dec")
	vars := []customMetricVariable{testCustomMetricCostA}
	withDecimals := testAccCustomMetricConfig(rName, "A", vars, `  decimals = { min = 2, max = 2 }`)
	without := testAccCustomMetricConfig(rName, "A", vars, "")
	withDecimalsRenamed := testAccCustomMetricConfig(rName+"-2", "A", vars, `  decimals = { min = 0, max = 4 }`)
	withoutRenamed := testAccCustomMetricConfig(rName+"-3", "A", vars, "")

	decimalsNull := statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.Null())

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with decimals
			{
				Config: withDecimals,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"min": knownvalue.Int64Exact(2),
						"max": knownvalue.Int64Exact(2),
					})),
				},
			},
			// Step 2: Drift check
			expectEmptyPlanStep(withDecimals),
			// Step 3: Remove decimals
			{
				Config: without,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testCustomMetricAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.Null()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{decimalsNull},
				Check:             testAccCheckCustomMetricDecimalsNullInAPI(t),
			},
			// Step 4: Drift check
			expectEmptyPlanStep(without),
			// Step 5: Set decimals again together with a rename
			{
				Config: withDecimalsRenamed,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"min": knownvalue.Int64Exact(0),
						"max": knownvalue.Int64Exact(4),
					})),
				},
			},
			// Step 6: Remove decimals together with a rename
			{
				Config: withoutRenamed,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testCustomMetricAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("decimals"), knownvalue.Null()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					decimalsNull,
					statecheck.ExpectKnownValue(testCustomMetricAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-3")),
				},
				Check: testAccCheckCustomMetricDecimalsNullInAPI(t),
			},
			// Step 7: Drift check
			expectEmptyPlanStep(withoutRenamed),
		},
	})
}

// An update whose only change comes from the decimals clearing modifier
// (Terraform proposed the stored value) must still plan etag and update_time
// unknown, or state keeps the old etag and the next update without refresh
// fails with 412.
func TestAccCustomMetric_ClearOnlyUpdateRefreshesEtag(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-cm-etag")
	vars := []customMetricVariable{testCustomMetricCostA}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: testAccCustomMetricConfig(rName, "A", vars, `  decimals = { min = 0, max = 2 }`)},
			{
				Config: testAccCustomMetricConfig(rName, "A", vars, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testCustomMetricAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue(testCustomMetricAddr, tfjsonpath.New("etag")),
						plancheck.ExpectUnknownValue(testCustomMetricAddr, tfjsonpath.New("update_time")),
					},
				},
				Check: testAccCheckCustomMetricEtagCurrent(t),
			},
		},
	})
}

// TestAccCustomMetric_Import imports a metric with every attribute set and a
// minimal one, and checks that configuration generated for each plans no change.
func TestAccCustomMetric_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-cm-import")
	full := testAccCustomMetricConfig(rName, "(A + 100) / B",
		[]customMetricVariable{testCustomMetricCostA, testCustomMetricUsageB},
		`  description = "Import test"
  unit_type   = "currency"
  decimals    = { min = 1, max = 2 }`)
	minimal := testAccCustomMetricConfig(rName+"-min", "A", []customMetricVariable{testCustomMetricCostA}, "")

	importSteps := func(config string) []resource.TestStep {
		return []resource.TestStep{
			{Config: config},
			{
				ResourceName:      testCustomMetricAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Configuration generated for the import (an import block with
			// -generate-config-out) is valid and plans no change.
			{
				ResourceName:    testCustomMetricAddr,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
			},
			expectEmptyPlanStep(config),
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps:                    append(importSteps(full), importSteps(minimal)...),
	})
}

func TestAccCustomMetric_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-cm-gone")
	config := testAccCustomMetricConfig(rName, "A", []customMetricVariable{testCustomMetricCostA}, "")
	var id, etag string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture its ID and ETag
			{
				Config: config,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources[testCustomMetricAddr]
					if !ok {
						return fmt.Errorf("%s not found in state", testCustomMetricAddr)
					}
					id, etag = rs.Primary.ID, rs.Primary.Attributes["etag"]
					return nil
				},
			},
			// Step 2: Delete it via the API, then verify Terraform plans to recreate it
			{
				PreConfig: func() {
					resp, err := getAPIClient(t).DeleteCustomMetricWithResponse(t.Context(), id, &models.DeleteCustomMetricParams{
						IfMatch:        etag,
						IdempotencyKey: uuid.NewV4().String(),
					})
					if err != nil {
						t.Fatalf("Failed to delete custom metric via API: %v", err)
					}
					if resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 204 or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccCustomMetric_Validation(t *testing.T) {
	config := func(variables int, metricType, decimals string) string {
		var vars strings.Builder
		for range variables {
			fmt.Fprintf(&vars, "    { metric = { type = %q, value = \"cost\" }, allocation_id = \"alloc\" },\n", metricType)
		}
		return fmt.Sprintf(`
resource "doit_custom_metric" "this" {
  name      = "validation"
  formula   = "A"
  variables = [
%s  ]
%s
}
`, vars.String(), decimals)
	}

	cases := []struct {
		name   string
		config string
		err    string
	}{
		{"decimals min greater than max", config(1, "basic", "  decimals = { min = 3, max = 1 }"), `Invalid Decimals Range`},
		{"six variables", config(6, "basic", ""), `list must contain at least 1 elements and at most 5`},
		{"no variables", config(0, "basic", ""), `list must contain at least 1 elements and at most 5`},
		{"custom metric as variable", config(1, "custom", ""), `value must be one of`},
		{"decimals above 5", config(1, "basic", "  decimals = { min = 0, max = 6 }"), `must be between 0 and 5`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config:      tc.config,
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(tc.err),
					},
				},
			})
		})
	}
}

// testAccCheckCustomMetricEtagCurrent checks that the etag in state is the one
// the API returns now, so the next update's If-Match succeeds without refresh.
func testAccCheckCustomMetricEtagCurrent(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		metric, err := testAccGetCustomMetric(t, s)
		if err != nil {
			return err
		}
		if got := s.RootModule().Resources[testCustomMetricAddr].Primary.Attributes["etag"]; metric.Etag == nil || got != *metric.Etag {
			return fmt.Errorf("etag in state %q, API returns %v", got, metric.Etag)
		}
		return nil
	}
}

// testAccCheckCustomMetricDecimalsNullInAPI checks that the API stores no
// display precision, not only that state says so.
func testAccCheckCustomMetricDecimalsNullInAPI(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		metric, err := testAccGetCustomMetric(t, s)
		if err != nil {
			return err
		}
		if d, err := metric.Decimals.Get(); err == nil {
			return fmt.Errorf("API still returns decimals %+v", d)
		}
		return nil
	}
}

func testAccGetCustomMetric(t *testing.T, s *terraform.State) (*models.CustomMetric, error) {
	rs, ok := s.RootModule().Resources[testCustomMetricAddr]
	if !ok {
		return nil, fmt.Errorf("%s not found in state", testCustomMetricAddr)
	}
	resp, err := getAPIClient(t).GetCustomMetricWithResponse(t.Context(), rs.Primary.ID, &models.GetCustomMetricParams{})
	if err != nil {
		return nil, fmt.Errorf("reading custom metric: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("reading custom metric: status %d, body: %s", resp.StatusCode(), string(resp.Body))
	}
	return resp.JSON200, nil
}
