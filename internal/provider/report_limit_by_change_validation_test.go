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

// lbcSpec describes one doit_report / doit_report_query config used by the
// limit_by_change validation tests. Metrics and the limit_by_change metric are
// rendered as HCL object bodies, e.g. `type = "basic", value = "cost"`.
type lbcSpec struct {
	aggregation     string
	count           bool
	secondary       bool
	limitByChange   bool
	metrics         []string
	limitMetric     string
	limitType       string // defaults to basic
	aggregationExpr string // overrides aggregation with a raw HCL expression
	limitValueExpr  string // overrides the limit_by_change metric value with a raw HCL expression
	prelude         string // extra HCL emitted before the resource, e.g. terraform_data
}

func lbcMetricHCL(typ, value string) string {
	return fmt.Sprintf(`{ type = %q, value = %q }`, typ, value)
}

func (s lbcSpec) configBody() string {
	body := "metrics = ["
	for i, m := range s.metrics {
		if i > 0 {
			body += ", "
		}
		body += m
	}
	body += "]\n"

	if s.aggregationExpr != "" {
		body += fmt.Sprintf("aggregation = %s\n", s.aggregationExpr)
	} else {
		body += fmt.Sprintf("aggregation = %q\n", s.aggregation)
	}
	if s.count {
		body += "count = { id = \"service_description\", type = \"fixed\" }\n"
	}
	body += `time_interval  = "month"
data_source    = "billing"
display_values = "actuals_only"
currency       = "USD"
layout         = "table"
time_range = {
  mode            = "last"
  amount          = 3
  unit            = "month"
  include_current = true
}
`
	if s.secondary {
		body += `secondary_time_range = {
  amount          = 1
  unit            = "year"
  include_current = false
}
`
	}
	if s.limitByChange {
		limitType := s.limitType
		if limitType == "" {
			limitType = "basic"
		}
		value := fmt.Sprintf("%q", s.limitMetric)
		if s.limitValueExpr != "" {
			value = s.limitValueExpr
		}
		body += fmt.Sprintf(`limit_by_change = {
  metric = {
    type  = %q
    value = %s
  }
  change_type             = "percentage"
  operator                = "gte"
  values                  = [50]
  include_incomplete_data = false
}
`, limitType, value)
	}
	return body
}

func (s lbcSpec) report(i int) string {
	return fmt.Sprintf(`
%s
resource "doit_report" "lbc_validation" {
  name = "test-lbc-validation-%d"
  config = {
%s
  }
}
`, s.prelude, i, s.configBody())
}

func (s lbcSpec) query() string {
	return fmt.Sprintf(`
%s
data "doit_report_query" "lbc_validation" {
  config = {
%s
  }
}
`, s.prelude, s.configBody())
}

var (
	lbcCostMetric     = lbcMetricHCL("basic", "cost")
	lbcExtendedMetric = lbcMetricHCL("extended", "amortized_savings")
)

const (
	lbcCountError  = `Invalid Limit By Change Configuration`
	lbcMetricError = `Invalid Limit By Change Metric`
)

// TestAccReport_LimitByChange_SecondaryRange verifies that limit_by_change with a
// secondary time range is accepted and stable across re-apply.
func TestAccReport_LimitByChange_SecondaryRange(t *testing.T) {
	n := acctest.RandInt()
	spec := lbcSpec{aggregation: "total", secondary: true, limitByChange: true, metrics: []string{lbcCostMetric}, limitMetric: "cost"}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: spec.report(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.lbc_validation",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("metric").AtMapKey("value"),
						knownvalue.StringExact("cost")),
					statecheck.ExpectKnownValue("doit_report.lbc_validation",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("unit"),
						knownvalue.StringExact("year")),
				},
			},
			{
				Config: spec.report(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_LimitByChange_SecondaryRange_Extended verifies an extended
// metric as the limit_by_change metric next to other metrics.
func TestAccReport_LimitByChange_SecondaryRange_Extended(t *testing.T) {
	n := acctest.RandInt()
	spec := lbcSpec{
		aggregation:   "total",
		secondary:     true,
		limitByChange: true,
		metrics:       []string{lbcCostMetric, lbcExtendedMetric},
		limitMetric:   "amortized_savings",
		limitType:     "extended",
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: spec.report(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.lbc_validation",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("metric").AtMapKey("type"),
						knownvalue.StringExact("extended")),
				},
			},
			{
				Config: spec.report(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_CountWithSecondaryRange verifies the validator does not reject
// count aggregation with a secondary range when limit_by_change is not set.
func TestAccReport_CountWithSecondaryRange(t *testing.T) {
	n := acctest.RandInt()
	spec := lbcSpec{aggregation: "count", count: true, secondary: true, metrics: []string{lbcCostMetric}}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: spec.report(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.lbc_validation",
						tfjsonpath.New("config").AtMapKey("aggregation"),
						knownvalue.StringExact("count")),
				},
			},
			{
				Config: spec.report(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_LimitByChange_CountWithSecondaryRejected verifies that count
// aggregation combined with limit_by_change and a secondary range is rejected at plan time.
func TestAccReport_LimitByChange_CountWithSecondaryRejected(t *testing.T) {
	n := acctest.RandInt()
	spec := lbcSpec{aggregation: "count", count: true, secondary: true, limitByChange: true, metrics: []string{lbcCostMetric}, limitMetric: "cost"}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      spec.report(n),
				ExpectError: regexp.MustCompile(lbcCountError),
			},
		},
	})
}

// TestAccReport_LimitByChange_MetricNotListedRejected verifies that a
// limit_by_change metric missing from metrics is rejected at plan time.
func TestAccReport_LimitByChange_MetricNotListedRejected(t *testing.T) {
	n := acctest.RandInt()
	spec := lbcSpec{aggregation: "total", secondary: true, limitByChange: true, metrics: []string{lbcCostMetric}, limitMetric: "usage"}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      spec.report(n),
				ExpectError: regexp.MustCompile(lbcMetricError),
			},
		},
	})
}

// TestAccReport_LimitByChange_MetricNotListedRejectedDespiteUnknownEntry verifies
// that a metrics entry whose value is unknown at plan time, but whose known type
// already differs from the limit_by_change metric, does not hide the error.
func TestAccReport_LimitByChange_MetricNotListedRejectedDespiteUnknownEntry(t *testing.T) {
	n := acctest.RandInt()
	prelude := `
resource "terraform_data" "metric_id" {
  input = "some-custom-metric-id"
}
`
	spec := lbcSpec{
		aggregation:   "total",
		secondary:     true,
		limitByChange: true,
		metrics:       []string{lbcCostMetric, `{ type = "custom", value = terraform_data.metric_id.output }`},
		limitMetric:   "usage",
		prelude:       prelude,
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      spec.report(n),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(lbcMetricError),
			},
		},
	})
}

// TestAccReport_LimitByChange_ValidatorDefersUnknowns verifies that values that
// are unknown at plan time (here fed by terraform_data outputs) do not trigger
// the limit_by_change validators.
func TestAccReport_LimitByChange_ValidatorDefersUnknowns(t *testing.T) {
	n := acctest.RandInt()
	prelude := `
resource "terraform_data" "aggregation" {
  input = "total"
}
resource "terraform_data" "limit_metric" {
  input = "cost"
}
`
	spec := lbcSpec{
		aggregationExpr: "terraform_data.aggregation.output",
		secondary:       true,
		limitByChange:   true,
		metrics:         []string{lbcCostMetric},
		limitMetric:     "cost",
		limitValueExpr:  "terraform_data.limit_metric.output",
		prelude:         prelude,
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:             spec.report(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccReportQueryDataSource_LimitByChange_Rejected verifies the report_query
// data source applies the same limit_by_change rules at plan time.
func TestAccReportQueryDataSource_LimitByChange_Rejected(t *testing.T) {
	countSpec := lbcSpec{aggregation: "count", count: true, secondary: true, limitByChange: true, metrics: []string{lbcCostMetric}, limitMetric: "cost"}
	notListedSpec := lbcSpec{aggregation: "total", secondary: true, limitByChange: true, metrics: []string{lbcCostMetric}, limitMetric: "usage"}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      countSpec.query(),
				ExpectError: regexp.MustCompile(lbcCountError),
			},
			{
				Config:      notListedSpec.query(),
				ExpectError: regexp.MustCompile(lbcMetricError),
			},
		},
	})
}
