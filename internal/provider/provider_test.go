package provider_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// getAPIClient creates an API client for test helpers that need to call
// the API directly (e.g., counting resources, deleting a resource mid-test).
func getAPIClient(t *testing.T) *models.ClientWithResponses {
	t.Helper()
	client, err := provider.NewClient(
		context.Background(),
		os.Getenv("DOIT_HOST"),
		os.Getenv("DOIT_API_TOKEN"),
		os.Getenv("DOIT_CUSTOMER_CONTEXT"),
		"test", // terraformVersion - not available outside provider lifecycle
		"dev",  // providerVersion - matches default in main.go
	)
	if err != nil {
		t.Fatalf("Failed to create API client: %v", err)
	}
	return client
}

// testCheckResourceAttrLessThan returns a TestCheckFunc that verifies the given
// integer attribute is strictly less than the threshold. Used to prove that
// page_token-only auto-pagination starts from the token (fewer results than a full run).
func testCheckResourceAttrLessThan(name, key string, threshold int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found", name)
		}
		val, ok := rs.Primary.Attributes[key]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", key, name)
		}
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("attribute %s on %s is not an integer: %s", key, name, val)
		}
		if intVal >= threshold {
			return fmt.Errorf("expected %s on %s to be less than %d, got %d", key, name, threshold, intVal)
		}
		return nil
	}
}
