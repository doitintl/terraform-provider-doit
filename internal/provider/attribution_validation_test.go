package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The CI tenant has no DoiT Attribute plan, so these tests cannot run a real
// attribution query. They check that the generated schemas reject the Attribute
// values where the API cannot use them, and that a valid attribution query
// reaches the API. Only TestAccReportQuery_AttributionReachesAPI calls the API.

const attributionTestReportConfig = `
resource "doit_report" "this" {
  name = "test-attribution-rejected"
  config = {
    data_source   = %q
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 7
      unit            = "day"
      include_current = true
    }
    group = [{ id = "set_a", type = %q }]
  }
}
`

func TestAccReport_AttributionDataSourceRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config:      fmt.Sprintf(attributionTestReportConfig, "attribution", "fixed"),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value Match.*got:\s+"attribution"`),
		}},
	})
}

func TestAccReport_AttributionTierRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config:      fmt.Sprintf(attributionTestReportConfig, "billing", "attribution_tier_1"),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value Match.*got:\s+"attribution_tier_1"`),
		}},
	})
}

func TestAccAlert_AttributionTierRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config: `
resource "doit_alert" "this" {
  name = "test-attribution-tier-rejected"
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
    scopes = [{ id = "set_a", type = "attribution_tier_2", mode = "is", values = ["team-a"] }]
  }
}
`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value Match.*got:\s+"attribution_tier_2"`),
		}},
	})
}

func TestAccBudget_AttributionTierRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config: `
resource "doit_budget" "this" {
  name          = "test-attribution-tier-rejected"
  currency      = "USD"
  amount        = 100
  type          = "recurring"
  time_interval = "month"
  start_period  = 1767225600000
  scopes = [{ id = "set_a", type = "attribution_tier_3", mode = "is", values = ["team-a"] }]
}
`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value Match.*got:\s+"attribution_tier_3"`),
		}},
	})
}

const attributionTestQueryConfig = `
data "doit_report_query" "this" {
  config = {
    %s
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 7
      unit            = "day"
      include_current = true
    }
    group = [%s]
  }
}
`

func TestAccReportQuery_AttributionValidation(t *testing.T) {
	steps := []struct {
		dataSource string
		group      string
		wantError  string
	}{
		{
			// Tier dimensions without the attribution data source.
			group:     `{ id = "set_a", type = "attribution_tier_1" }`,
			wantError: `require\s+` + "`" + `data_source = "attribution"` + "`",
		},
		{
			// Attribution data source without a tier dimension.
			dataSource: `data_source = "attribution"`,
			group:      `{ id = "service_description", type = "fixed" }`,
			wantError:  `requires at least one\s+` + "`" + `attribution_tier_1` + "`",
		},
		{
			// Tier dimensions of two attributions.
			dataSource: `data_source = "attribution"`,
			group:      `{ id = "set_a", type = "attribution_tier_1" }, { id = "set_b", type = "attribution_tier_2" }`,
			wantError:  `must belong to the same\s+attribution`,
		},
	}

	testSteps := make([]resource.TestStep, 0, len(steps))
	for _, step := range steps {
		testSteps = append(testSteps, resource.TestStep{
			Config:      fmt.Sprintf(attributionTestQueryConfig, step.dataSource, step.group),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid Attribution Query Configuration(.|\n)*` + step.wantError),
		})
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps:                    testSteps,
	})
}

// TestAccReportQuery_AttributionReachesAPI checks that a valid attribution
// query passes provider validation and is sent to the API, which rejects it
// because the CI tenant has no Attribute plan. If the CI tenant ever gets one,
// replace this with a test that reads real attribution results.
func TestAccReportQuery_AttributionReachesAPI(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(attributionTestQueryConfig,
				`data_source = "attribution"`,
				`{ id = "set_a", type = "attribution_tier_1" }`),
			ExpectError: regexp.MustCompile(`(?i)entitlement_not_enabled|enable\s+Attribute`),
		}},
	})
}

// TestAccReport_OmittedDataSourceIsUnknownOnUpdate checks the plan behavior the
// ModifyPlan guard for attribution reports relies on: when config omits
// data_source and another attribute changes, the plan holds an unknown value,
// not the stored one. The guard therefore reads the stored value when config
// omits data_source. See TestReportResource_ModifyPlan_AttributionReportReadOnly.
func TestAccReport_OmittedDataSourceIsUnknownOnUpdate(t *testing.T) {
	n := acctest.RandInt()
	config := func(currency string) string {
		return fmt.Sprintf(`
resource "doit_report" "test" {
  name = "test-omitted-data-source-%d"
  config = {
    currency      = %q
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 7
      unit            = "day"
      include_current = true
    }
    metrics = [{ type = "basic", value = "cost" }]
  }
}
`, n, currency)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config("USD")},
			{
				Config: config("EUR"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue("doit_report.test", tfjsonpath.New("config").AtMapKey("data_source")),
					},
				},
			},
		},
	})
}
