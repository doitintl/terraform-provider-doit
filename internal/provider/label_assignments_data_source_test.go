package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccLabelAssignmentsDataSource_Basic creates a label and verifies that
// the label_assignments data source returns an empty assignments list for it.
func TestAccLabelAssignmentsDataSource_Basic(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelAssignmentsBasic(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.ListExact([]knownvalue.Check{}), // empty list
					),
				},
			},
		},
	})
}

func testAccLabelAssignmentsBasic(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-assign-basic-%d"
  color = "blue"
}

data "doit_label_assignments" "test" {
  id = doit_label.test.id
}
`, i)
}

// TestAccLabelAssignmentsDataSource_WithReport creates a label and a report
// that is assigned to the label, then verifies the label_assignments data
// source returns the report in the assignments list.
func TestAccLabelAssignmentsDataSource_WithReport(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelAssignmentsWithReport(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify assignments list has exactly one entry
					statecheck.ExpectKnownValue(
						"data.doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.ListSizeExact(1),
					),
					// Verify the assignment's object_id matches the report
					statecheck.ExpectKnownValue(
						"data.doit_label_assignments.test",
						tfjsonpath.New("assignments").AtSliceIndex(0).AtMapKey("object_type"),
						knownvalue.StringExact("report"),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					// The object_id in assignments should match the report ID
					resource.TestCheckResourceAttrPair(
						"data.doit_label_assignments.test", "assignments.0.object_id",
						"doit_report.test", "id",
					),
				),
			},
		},
	})
}

func testAccLabelAssignmentsWithReport(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-assign-report-%d"
  color = "teal"
}

resource "doit_report" "test" {
  name        = "tf-acc-labeled-report-%d"
  description = "Report for label assignment test"
  labels      = [doit_label.test.id]
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    aggregation    = "total"
    time_interval  = "month"
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}

data "doit_label_assignments" "test" {
  id         = doit_label.test.id
  depends_on = [doit_report.test]
}
`, i, i)
}

// TestAccLabelAssignmentsDataSource_NotFound verifies that querying a
// non-existent label ID returns an error.
func TestAccLabelAssignmentsDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelAssignmentsNotFound(),
				ExpectError: regexp.MustCompile(`(?i)error reading label assignments|not found|404`),
			},
		},
	})
}

func testAccLabelAssignmentsNotFound() string {
	return `
data "doit_label_assignments" "test" {
  id = "non-existent-label-id-00000000"
}
`
}
