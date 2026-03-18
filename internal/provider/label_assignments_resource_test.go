package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccLabelAssignmentsResource_Basic creates a label and a report,
// assigns the report to the label, and verifies the assignment.
func TestAccLabelAssignmentsResource_Basic(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelAssignmentsResourceBasic(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"doit_label_assignments.test", "label_id",
						"doit_label.test", "id",
					),
					resource.TestCheckResourceAttrPair(
						"doit_label_assignments.test", "id",
						"doit_label.test", "id",
					),
				),
			},
			// Verify import
			{
				ResourceName:      "doit_label_assignments.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLabelAssignmentsResourceBasic(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-assignments-basic-%d"
  color = "lavender"
}

resource "doit_report" "test" {
  name        = "tf-acc-assignments-report-%d"
  description = "Report for label assignments resource test"
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

resource "doit_label_assignments" "test" {
  label_id = doit_label.test.id
  assignments = [
    {
      object_id   = doit_report.test.id
      object_type = "report"
    },
  ]
}
`, i, i)
}

// TestAccLabelAssignmentsResource_Update creates assignments, then adds a second
// report and verifies the diff-based update works correctly.
func TestAccLabelAssignmentsResource_Update(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: One assignment
			{
				Config: testAccLabelAssignmentsResourceOneReport(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
			// Step 2: Two assignments (add a second report)
			{
				Config: testAccLabelAssignmentsResourceTwoReports(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(2),
					),
				},
			},
			// Step 3: Back to one assignment (remove the second)
			{
				Config: testAccLabelAssignmentsResourceOneReport(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
		},
	})
}

func testAccLabelAssignmentsResourceOneReport(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-assignments-update-%d"
  color = "skyBlue"
}

resource "doit_report" "test1" {
  name        = "tf-acc-assignments-report1-%d"
  description = "First report for update test"
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

resource "doit_report" "test2" {
  name        = "tf-acc-assignments-report2-%d"
  description = "Second report for update test"
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

resource "doit_label_assignments" "test" {
  label_id = doit_label.test.id
  assignments = [
    {
      object_id   = doit_report.test1.id
      object_type = "report"
    },
  ]
}
`, i, i, i)
}

func testAccLabelAssignmentsResourceTwoReports(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-assignments-update-%d"
  color = "skyBlue"
}

resource "doit_report" "test1" {
  name        = "tf-acc-assignments-report1-%d"
  description = "First report for update test"
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

resource "doit_report" "test2" {
  name        = "tf-acc-assignments-report2-%d"
  description = "Second report for update test"
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

resource "doit_label_assignments" "test" {
  label_id = doit_label.test.id
  assignments = [
    {
      object_id   = doit_report.test1.id
      object_type = "report"
    },
    {
      object_id   = doit_report.test2.id
      object_type = "report"
    },
  ]
}
`, i, i, i)
}

// TestAccLabelAssignmentsResource_MixedObjectTypes assigns different object types
// (report, budget, alert) to the same label and verifies they all work together.
func TestAccLabelAssignmentsResource_MixedObjectTypes(t *testing.T) {
	n := acctest.RandInt()
	attribution := os.Getenv("TEST_ATTRIBUTION")
	if attribution == "" {
		t.Skip("TEST_ATTRIBUTION must be set for this test")
	}
	testUserEmail := os.Getenv("TEST_USER")
	if testUserEmail == "" {
		t.Skip("TEST_USER must be set for this test")
	}

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelAssignmentsResourceMixedTypes(n, attribution, testUserEmail),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(3),
					),
				},
			},
			// Verify import for mixed types
			{
				ResourceName:      "doit_label_assignments.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLabelAssignmentsResourceMixedTypes(i int, attribution string, testUserEmail string) string {
	return fmt.Sprintf(`
%s

resource "doit_label" "test" {
  name  = "tf-acc-assignments-mixed-%d"
  color = "apricot"
}

resource "doit_report" "test" {
  name        = "tf-acc-mixed-report-%d"
  description = "Report for mixed type test"
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

resource "doit_budget" "test" {
  name          = "tf-acc-mixed-budget-%d"
  amount        = 100
  currency      = "USD"
  time_interval = "month"
  scope         = ["%s"]
  type          = "recurring"
  start_period  = local.start_period
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
}

resource "doit_alert" "test" {
  name = "tf-acc-mixed-alert-%d"
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

resource "doit_label_assignments" "test" {
  label_id = doit_label.test.id
  assignments = [
    {
      object_id   = doit_report.test.id
      object_type = "report"
    },
    {
      object_id   = doit_budget.test.id
      object_type = "budget"
    },
    {
      object_id   = doit_alert.test.id
      object_type = "alert"
    },
  ]
}
`, budgetStartPeriod(), i, i, i, attribution, testUserEmail, i)
}

// TestAccLabelAssignmentsResource_AllocationAssignment tests assigning an allocation
// to a label.
func TestAccLabelAssignmentsResource_AllocationAssignment(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelAssignmentsResourceAllocation(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
		},
	})
}

func testAccLabelAssignmentsResourceAllocation(i int) string {
	project := os.Getenv("TEST_PROJECT")

	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-assignments-alloc-%d"
  color = "rosePink"
}

resource "doit_allocation" "test" {
  name        = "tf-acc-alloc-assign-%d"
  description = "Allocation for label assignment test"
  rule = {
    formula    = "A"
    components = [
      {
        key    = "project_id"
        type   = "fixed"
        mode   = "is"
        values = ["%s"]
      },
    ]
  }
}

resource "doit_label_assignments" "test" {
  label_id = doit_label.test.id
  assignments = [
    {
      object_id   = doit_allocation.test.id
      object_type = "allocation"
    },
  ]
}
`, i, i, project)
}

// TestAccLabelAssignmentsResource_AnnotationAssignment tests assigning an annotation
// to a label.
func TestAccLabelAssignmentsResource_AnnotationAssignment(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelAssignmentsResourceAnnotation(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
		},
	})
}

func testAccLabelAssignmentsResourceAnnotation(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-assignments-annot-%d"
  color = "teal"
}

resource "doit_annotation" "test" {
  content   = "tf-acc-annotation-assign-%d"
  timestamp = "2024-06-01T00:00:00Z"
}

resource "doit_label_assignments" "test" {
  label_id = doit_label.test.id
  assignments = [
    {
      object_id   = doit_annotation.test.id
      object_type = "annotation"
    },
  ]
}
`, i, i)
}

// TestAccLabelAssignmentsResource_ReportLabelsConflict tests the interaction between
// the report resource's `labels` attribute and the `doit_label_assignments` resource.
//
// Scenario: A report is created with a label assigned via the report's `labels`
// attribute, and separately the SAME label is also managed by doit_label_assignments.
// This tests whether both mechanisms can coexist or if they cause drift/errors.
func TestAccLabelAssignmentsResource_ReportLabelsConflict(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create report with labels attribute AND doit_label_assignments
			// managing the SAME label assignment. This tests whether both can apply
			// without error and whether Read detects drift.
			{
				Config: testAccLabelAssignmentsResourceReportWithSameLabel(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
				// Verify the report also shows the label
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						"doit_report.test", "labels.#", "1",
					),
				),
			},
			// Step 2: Re-apply the same config. If there's drift between
			// the two resources, this step will fail with "inconsistent result"
			// or require changes when none are expected.
			{
				Config:   testAccLabelAssignmentsResourceReportWithSameLabel(n),
				PlanOnly: true,
			},
		},
	})
}

func testAccLabelAssignmentsResourceReportWithSameLabel(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "tf-acc-conflict-label-%d"
  color = "mint"
}

resource "doit_report" "test" {
  name        = "tf-acc-conflict-report-%d"
  description = "Report with labels attribute for conflict test"
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

resource "doit_label_assignments" "test" {
  label_id   = doit_label.test.id
  assignments = [
    {
      object_id   = doit_report.test.id
      object_type = "report"
    },
  ]
  depends_on = [doit_report.test]
}
`, i, i)
}

// TestAccLabelAssignmentsResource_ReportLabelsDifferent tests the scenario where
// a report has one label set via its `labels` attribute and a DIFFERENT label is
// managed via doit_label_assignments.
//
// KNOWN BEHAVIOR: This causes drift! When doit_label_assignments assigns a
// different label to the report, the API adds it to the report's labels list.
// On refresh, the report resource sees an extra label it didn't configure and
// plans to remove it. This test documents this behavior to warn users against
// mixing both mechanisms with DIFFERENT labels on the same report.
func TestAccLabelAssignmentsResource_ReportLabelsDifferent(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelAssignmentsResourceReportWithDifferentLabel(n),
				// ExpectNonEmptyPlan tells the framework to accept drift on
				// post-apply refresh. Without this, the test fails because
				// the report resource detects the extra label added by
				// doit_label_assignments and plans to remove it.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccLabelAssignmentsResourceReportWithDifferentLabel(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "via_report" {
  name  = "tf-acc-via-report-%d"
  color = "softYellow"
}

resource "doit_label" "via_assignments" {
  name  = "tf-acc-via-assignments-%d"
  color = "purple"
}

resource "doit_report" "test" {
  name        = "tf-acc-diff-label-report-%d"
  description = "Report with different labels for conflict test"
  labels      = [doit_label.via_report.id]
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

resource "doit_label_assignments" "test" {
  label_id   = doit_label.via_assignments.id
  assignments = [
    {
      object_id   = doit_report.test.id
      object_type = "report"
    },
  ]
  depends_on = [doit_report.test]
}
`, i, i, i)
}

// TestAccLabelAssignmentsResource_ReportLabelsAddedLater tests the reverse
// dependency: a report is created WITHOUT labels, then a label is assigned via
// doit_label_assignments, and THEN the report is updated to add a label via
// its `labels` attribute. This reveals whether the report's update overwrites
// the assignment-managed label.
func TestAccLabelAssignmentsResource_ReportLabelsAddedLater(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create report (no labels) + assign via doit_label_assignments
			{
				Config: testAccLabelAssignmentsResourceReportNoLabels(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
			// Step 2: Update report to add a DIFFERENT label via `labels` attr.
			// Does the report's update overwrite/remove the assignment label?
			{
				Config: testAccLabelAssignmentsResourceReportAddDifferentLabel(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// The assignments resource should still have 1 assignment
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
				// Expect drift: report will see the assignment label on refresh
				// and plan to remove it (same behavior as ReportLabelsDifferent).
				ExpectNonEmptyPlan: true,
			},
			// Step 3: Update report to use the SAME label as the assignments
			// resource. This should be stable (no drift).
			{
				Config: testAccLabelAssignmentsResourceReportAddSameLabel(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label_assignments.test",
						tfjsonpath.New("assignments"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
		},
	})
}

// Step 1: Report without labels, label assigned via assignments resource.
func testAccLabelAssignmentsResourceReportNoLabels(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "via_assignments" {
  name  = "tf-acc-reverse-assign-%d"
  color = "apricot"
}

resource "doit_label" "via_report" {
  name  = "tf-acc-reverse-report-%d"
  color = "purple"
}

resource "doit_report" "test" {
  name        = "tf-acc-reverse-dep-report-%d"
  description = "Report created without labels for reverse dependency test"
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

resource "doit_label_assignments" "test" {
  label_id   = doit_label.via_assignments.id
  assignments = [
    {
      object_id   = doit_report.test.id
      object_type = "report"
    },
  ]
}
`, i, i, i)
}

// Step 2: Report updated to add a DIFFERENT label via `labels` attribute.
func testAccLabelAssignmentsResourceReportAddDifferentLabel(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "via_assignments" {
  name  = "tf-acc-reverse-assign-%d"
  color = "apricot"
}

resource "doit_label" "via_report" {
  name  = "tf-acc-reverse-report-%d"
  color = "purple"
}

resource "doit_report" "test" {
  name        = "tf-acc-reverse-dep-report-%d"
  description = "Report updated to add labels for reverse dependency test"
  labels      = [doit_label.via_report.id]
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

resource "doit_label_assignments" "test" {
  label_id   = doit_label.via_assignments.id
  assignments = [
    {
      object_id   = doit_report.test.id
      object_type = "report"
    },
  ]
}
`, i, i, i)
}

// Step 3: Report updated to use the SAME label as assignments resource.
func testAccLabelAssignmentsResourceReportAddSameLabel(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "via_assignments" {
  name  = "tf-acc-reverse-assign-%d"
  color = "apricot"
}

resource "doit_label" "via_report" {
  name  = "tf-acc-reverse-report-%d"
  color = "purple"
}

resource "doit_report" "test" {
  name        = "tf-acc-reverse-dep-report-%d"
  description = "Report updated to use same label as assignments"
  labels      = [doit_label.via_assignments.id]
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

resource "doit_label_assignments" "test" {
  label_id   = doit_label.via_assignments.id
  assignments = [
    {
      object_id   = doit_report.test.id
      object_type = "report"
    },
  ]
}
`, i, i, i)
}

// values are rejected at plan time.
func TestAccLabelAssignmentsResource_InvalidType(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
resource "doit_label_assignments" "test" {
  label_id = "some-label-id"
  assignments = [
    {
      object_id   = "some-object-id"
      object_type = "invalid_type"
    },
  ]
}
`,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}
