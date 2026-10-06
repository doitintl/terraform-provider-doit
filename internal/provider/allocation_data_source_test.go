package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAllocationDataSource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alloc-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify data source attributes match resource
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("test allocation for data source")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("single")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("rule").AtMapKey("formula"),
						knownvalue.StringExact("A")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("anomaly_detection"),
						knownvalue.Bool(true)),
				},
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAllocationDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccAllocationDataSource_Group(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alloc-ds-grp")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationDataSourceGroupConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("allocation_type"),
						knownvalue.StringExact("group")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("unallocated_costs"),
						knownvalue.StringExact("Other")),
				},
			},
		},
	})
}

func testAccAllocationDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "test" {
    name              = %q
    description       = "test allocation for data source"
    anomaly_detection = true
    rule = {
       formula = "A"
       components = [
        {
           key    = "project_id"
           mode   = "is"
           type   = "fixed"
           values = ["%s"]
         }
       ]
    }
}

data "doit_allocation" "test" {
    id = doit_allocation.test.id
}
`, name, testProject())
}

func testAccAllocationDataSourceGroupConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "test" {
    name        = %q
    description = "test allocation group for data source"
    unallocated_costs = "Other"
    rules = [
        {
            action  = "create"
            name    = "Group 1"
            formula = "A"
            components = [
             {
                key    = "project_id"
                mode   = "is"
                type   = "fixed"
                values = ["%s"]
              }
            ]
        }
    ]
}

data "doit_allocation" "test" {
    id = doit_allocation.test.id
}
`, name, testProject())
}

func TestAccAllocationDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAllocationDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(not found|404|Not Found)`),
			},
		},
	})
}

func testAccAllocationDataSourceNotFoundConfig() string {
	return `
data "doit_allocation" "test" {
    id = "non-existent-allocation-id"
}
`
}

// TestAccAllocationDataSource_FolderId verifies that the data source returns the
// correct folder_id when an allocation is created inside a folder.
func TestAccAllocationDataSource_FolderId(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alloc-ds-f")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationDataSourceFolderIdConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_allocation.test", "folder_id",
						"doit_folder.ds_alloc_test", "id"),
				),
			},
		},
	})
}

func testAccAllocationDataSourceFolderIdConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "ds_alloc_test" {
    name = "%s-folder"
}

resource "doit_allocation" "test" {
    name        = %q
    description = "Folder DS test allocation"
    folder_id   = doit_folder.ds_alloc_test.id
    rule = {
       formula = "A"
       components = [
        {
           key    = "project_id"
           mode   = "is"
           type   = "fixed"
           values = ["%s"]
         }
       ]
    }
}

data "doit_allocation" "test" {
    id = doit_allocation.test.id
}
`, name, name, testProject())
}

func TestAccAllocationDataSource_ValidityPeriods(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alloc-ds-vp")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationDataSourceValidityPeriodsConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("rule").AtMapKey("validity_periods"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"start_date": knownvalue.Null(),
								"end_date":   knownvalue.StringExact("2025-06-30"),
							}),
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"start_date": knownvalue.StringExact("2025-07-01"),
								"end_date":   knownvalue.Null(),
							}),
						})),
				},
			},
			{
				Config: testAccAllocationDataSourceValidityPeriodsConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationDataSourceValidityPeriodsConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "test" {
  name        = %q
  description = "test allocation with validity periods for data source"
  rule = {
    formula = "A"
    components = [
      {
        key    = "country"
        mode   = "is"
        type   = "fixed"
        values = ["JP"]
      }
    ]
    validity_periods = [
      {
        end_date = "2025-06-30"
      },
      {
        start_date = "2025-07-01"
      }
    ]
  }
}

data "doit_allocation" "test" {
  id = doit_allocation.test.id
}
`, name)
}

func TestAccAllocationDataSource_Group_ValueExtraction(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alloc-ds-ve")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationDataSourceGroupValueExtractionConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("rules").AtSliceIndex(0).AtMapKey("validity_periods"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"start_date": knownvalue.StringExact("2025-01-01"),
								"end_date":   knownvalue.StringExact("2025-12-31"),
							}),
						})),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("rules").AtSliceIndex(0).AtMapKey("value_extraction").AtMapKey("on_missing"),
						knownvalue.StringExact("useFallback")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("rules").AtSliceIndex(0).AtMapKey("value_extraction").AtMapKey("fallback"),
						knownvalue.StringExact("DefaultEnv")),
					statecheck.ExpectKnownValue(
						"data.doit_allocation.test",
						tfjsonpath.New("rules").AtSliceIndex(0).AtMapKey("value_extraction").AtMapKey("sources"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type": knownvalue.StringExact("tag"),
								"key":  knownvalue.StringExact("Environment"),
								"providers": knownvalue.ListExact([]knownvalue.Check{
									knownvalue.StringExact("amazon-web-services"),
								}),
							}),
						})),
				},
			},
			{
				Config: testAccAllocationDataSourceGroupValueExtractionConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationDataSourceGroupValueExtractionConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "test" {
  name              = %q
  description       = "test allocation group with value extraction for data source"
  unallocated_costs = "Other"
  rules = [
    {
      action  = "create"
      name    = "Extracted Env"
      formula = "A"
      components = [
        {
          key    = "country"
          mode   = "is"
          type   = "fixed"
          values = ["JP"]
        }
      ]
      validity_periods = [
        {
          start_date = "2025-01-01"
          end_date   = "2025-12-31"
        }
      ]
      value_extraction = {
        on_missing = "useFallback"
        fallback   = "DefaultEnv"
        sources = [
          {
            type      = "tag"
            key       = "Environment"
            providers = ["amazon-web-services"]
          }
        ]
      }
    }
  ]
}

data "doit_allocation" "test" {
  id = doit_allocation.test.id
}
`, name)
}
