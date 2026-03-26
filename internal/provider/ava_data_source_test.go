package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAvaDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAvaDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ava.test", "answer"),
					resource.TestCheckResourceAttrSet("data.doit_ava.test", "id"),
					resource.TestCheckResourceAttr("data.doit_ava.test", "question", "What cloud providers am I using?"),
				),
			},
			// Note: No drift check step. Ava responses are non-deterministic,
			// so re-applying would always produce a diff on `answer`.
		},
	})
}

func testAccAvaDataSourceConfig() string {
	return `
data "doit_ava" "test" {
  question = "What cloud providers am I using?"
}
`
}
