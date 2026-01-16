package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAlertDataSource_Basic(t *testing.T) {
	alertID := os.Getenv("TEST_ALERT_ID")
	if alertID == "" {
		t.Skip("TEST_ALERT_ID environment variable not set")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertDataSourceConfig(alertID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alert.test", "id", alertID),
					resource.TestCheckResourceAttrSet("data.doit_alert.test", "name"),
				),
			},
		},
	})
}

func testAccAlertDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_alert" "test" {
  id = %[1]q
}
`, id)
}
