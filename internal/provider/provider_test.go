package provider_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestMain runs after all tests complete. It sweeps orphaned test allocations
// whose names start with testAllocPrefix. This must happen here (not in
// CheckDestroy) to avoid deleting allocations that belong to other parallel tests.
func TestMain(m *testing.M) {
	code := m.Run()

	// Only sweep during acceptance tests
	if os.Getenv("TF_ACC") != "" {
		sweepOrphanedAllocations()
	}

	os.Exit(code)
}

// sweepOrphanedAllocations deletes any allocations whose name starts with
// the test prefix. This catches orphaned child rule allocations that the API
// creates for group allocation rules but does not cascade-delete.
func sweepOrphanedAllocations() {
	host := os.Getenv("DOIT_HOST")
	token := os.Getenv("DOIT_API_TOKEN")
	customerCtx := os.Getenv("DOIT_CUSTOMER_CONTEXT")

	if host == "" || token == "" {
		return
	}

	client, err := provider.NewClient(
		context.Background(),
		host, token, customerCtx,
		"test", "dev",
	)
	if err != nil {
		log.Printf("Warning: sweep failed to create API client: %v", err)
		return
	}

	ctx := context.Background()
	sweepPrefix := testAllocPrefix + "-"
	var toDelete []string
	params := &models.ListAllocationsParams{}

	for {
		resp, err := client.ListAllocationsWithResponse(ctx, params)
		if err != nil {
			log.Printf("Warning: sweep failed to list allocations: %v", err)
			return
		}
		if resp.JSON200 == nil || resp.JSON200.Allocations == nil {
			break
		}
		for _, a := range *resp.JSON200.Allocations {
			if a.Name != nil && strings.HasPrefix(*a.Name, sweepPrefix) {
				toDelete = append(toDelete, *a.Id)
			}
		}
		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}

	for _, id := range toDelete {
		resp, err := client.DeleteAllocationWithResponse(ctx, id)
		if err != nil {
			log.Printf("Warning: sweep failed to delete allocation %s: %v", id, err)
			continue
		}
		if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
			log.Printf("Warning: sweep unexpected status %d deleting allocation %s", resp.StatusCode(), id)
		}
	}

	if len(toDelete) > 0 {
		log.Printf("Swept %d orphaned test allocation(s)", len(toDelete))
	}
}

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

// testCheckResourceAttrNotEqualAttr returns a TestCheckFunc that verifies a
// string attribute on one resource differs from a string attribute on another.
// Used to prove that page_token actually advances to a different page.
func testCheckResourceAttrNotEqualAttr(name1, key1, name2, key2 string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs1, ok := s.RootModule().Resources[name1]
		if !ok {
			return fmt.Errorf("resource %s not found", name1)
		}
		val1, ok := rs1.Primary.Attributes[key1]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", key1, name1)
		}

		rs2, ok := s.RootModule().Resources[name2]
		if !ok {
			return fmt.Errorf("resource %s not found", name2)
		}
		val2, ok := rs2.Primary.Attributes[key2]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", key2, name2)
		}

		if val1 == val2 {
			return fmt.Errorf("expected %s.%s (%s) to differ from %s.%s (%s)",
				name1, key1, val1, name2, key2, val2)
		}
		return nil
	}
}
