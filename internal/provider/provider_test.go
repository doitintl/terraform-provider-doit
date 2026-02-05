package provider_test

import (
	"context"
	"os"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

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

// testAccClient creates an API client for test helpers that need to manipulate
// resources directly via the API (e.g., deleting a resource mid-test).
func testAccClient(t *testing.T) *models.ClientWithResponses {
	t.Helper()
	client, err := provider.NewClient(
		context.Background(),
		os.Getenv("DOIT_HOST"),
		os.Getenv("DOIT_API_TOKEN"),
		os.Getenv("DOIT_CUSTOMER_CONTEXT"),
	)
	if err != nil {
		t.Fatalf("Failed to create API client: %v", err)
	}
	return client
}
