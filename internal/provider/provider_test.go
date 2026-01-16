package provider_test

import (
	"os"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

var (
	testAccProvidersProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
		"doit": providerserver.NewProtocol6WithError(provider.New("dev")()),
	}
	testAccTFVersionChecks = []tfversion.TerraformVersionCheck{
		tfversion.RequireAbove(tfversion.Version1_10_0),
	}
)

func testAccPreCheckFunc(t *testing.T) func() {
	return func() {
		if v := os.Getenv("DOIT_API_TOKEN"); v == "" {
			t.Fatal("DOIT_API_TOKEN must be set for acceptance tests")
		}
		if v := os.Getenv("DOIT_HOST"); v == "" {
			t.Fatal("DOIT_HOST must be explicitly set for acceptance tests")
		}
		if v := os.Getenv("TEST_USER"); v == "" {
			t.Fatal("TEST_USER must be set for acceptance tests")
		}
		if v := os.Getenv("TEST_ATTRIBUTION"); v == "" {
			t.Fatal("TEST_ATTRIBUTION must be set for acceptance tests")
		}
		if v := os.Getenv("TEST_SLACK_CHAN"); v == "" {
			t.Fatal("TEST_SLACK_CHAN must be set for acceptance tests")
		}
		if v := os.Getenv("TEST_PROJECT"); v == "" {
			t.Fatal("TEST_PROJECT must be set for acceptance tests")
		}
		if v := os.Getenv("TEST_CUSTOMER_ID"); v == "" {
			t.Fatal("TEST_CUSTOMER_ID must be set for acceptance tests")
		}
	}
}
