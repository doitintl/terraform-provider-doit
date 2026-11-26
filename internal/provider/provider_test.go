package provider_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"golang.org/x/sync/semaphore"
)

// TestMain runs after all tests complete. It sweeps orphaned test allocations
// owned by TEST_USER. This must happen here (not in CheckDestroy) to avoid
// deleting allocations that belong to other parallel tests.
func TestMain(m *testing.M) {
	code := m.Run()

	// The sweep is opt-in via TF_ACC_SWEEP, which only the acceptance targets
	// set. TF_ACC alone is not enough: it is often exported in a developer's
	// shell, where a targeted unit-test run must not touch the tenant.
	if os.Getenv("TF_ACC_SWEEP") != "" {
		sweepOrphanedAllocations()
	}

	os.Exit(code)
}

// sweepOrphanedAllocations deletes all allocations owned by TEST_USER.
// This catches both explicitly named test allocations (tfacc-*) and
// auto-named child allocations ("Group 1") that the API creates for
// group allocation rules but does not cascade-delete.
func sweepOrphanedAllocations() {
	host := os.Getenv("DOIT_HOST")
	token := os.Getenv("DOIT_API_TOKEN")
	customerCtx := os.Getenv("DOIT_CUSTOMER_CONTEXT")
	testUser := os.Getenv("TEST_USER")

	if host == "" || token == "" || testUser == "" {
		return
	}

	client, err := provider.NewClient(
		host, token, customerCtx,
		"test", "dev", provider.DefaultRequestTimeout,
	)
	if err != nil {
		log.Printf("Warning: sweep failed to create API client: %v", err)
		return
	}

	ctx := context.Background()
	ownerFilter := fmt.Sprintf("owner:%s", testUser)
	var toDelete []string
	params := &models.ListAllocationsParams{
		Filter: &ownerFilter,
	}

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
			if a.Owner != nil && *a.Owner == testUser {
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

// reportRunSlotCount is how many acceptance tests may run report data sources at
// the same time. The API allows each user 3 pending or running report
// operations, and every test shares one token.
const reportRunSlotCount = 3

// reportRunSlots holds one unit per acceptance test currently running a report.
var reportRunSlots = semaphore.NewWeighted(reportRunSlotCount)

// acquireReportRunSlots blocks until n units of sem are free and holds them
// until the test ends, destroy included. Weighted acquisition keeps two tests
// that each need several units from deadlocking on partial holds.
//
// This gate stands in for "one user at a time" across the test binary, where
// dozens of Terraform runs share one token and would otherwise start every
// report at once. It must not be the reason a single configuration works: the
// provider queues the reports of one Terraform run itself (reportRunLimiter),
// and the tests of that behavior take every unit, so the config runs alone
// against the API's cap.
func acquireReportRunSlots(t *testing.T, sem *semaphore.Weighted, n int64) {
	t.Helper()
	if err := sem.Acquire(t.Context(), n); err != nil {
		t.Fatalf("waiting for a report run slot: %v", err)
	}
	t.Cleanup(func() { sem.Release(n) })
}

// testAccReportPreCheckFunc is testAccPreCheckFunc for tests that run report
// data sources. It also takes one report run slot; see acquireReportRunSlots.
//
// PreCheck runs inside the test, after resource.ParallelTest has called
// t.Parallel(), so the tests still start together and only wait here.
func testAccReportPreCheckFunc(t *testing.T) func() {
	return gatedPreCheck(t, 1)
}

// testAccExclusiveReportPreCheckFunc is testAccReportPreCheckFunc for a test
// that needs the API's whole report cap to itself, so that what it checks is
// not helped or hindered by other tests' reports.
func testAccExclusiveReportPreCheckFunc(t *testing.T) func() {
	return gatedPreCheck(t, reportRunSlotCount)
}

func gatedPreCheck(t *testing.T, n int64) func() {
	check := testAccPreCheckFunc(t)
	var once sync.Once
	return func() {
		check()
		once.Do(func() { acquireReportRunSlots(t, reportRunSlots, n) })
	}
}

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
		if v := os.Getenv("TEST_PROJECT"); v == "" {
			t.Fatal("TEST_PROJECT must be set for acceptance tests")
		}
		if v := os.Getenv("TEST_CUSTOMER_ID"); v == "" {
			t.Fatal("TEST_CUSTOMER_ID must be set for acceptance tests")
		}
	}
}

// skipIfNoAcc skips the calling test immediately if acceptance tests are
// disabled. resource.Test/ParallelTest already gate on TF_ACC internally, but
// sync.Once-cached count helpers (e.g. getAlertCount) run before that gate is
// reached, making a real API call even during a plain `make test` run. Call
// this as the first line of any such helper to skip before hitting the
// network instead of failing or hanging without credentials.
func skipIfNoAcc(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}
}

// getAPIClient creates an API client for test helpers that need to call
// the API directly (e.g., counting resources, deleting a resource mid-test).
func getAPIClient(t *testing.T) *models.ClientWithResponses {
	t.Helper()
	skipIfNoAcc(t)
	client, err := provider.NewClient(
		os.Getenv("DOIT_HOST"),
		os.Getenv("DOIT_API_TOKEN"),
		os.Getenv("DOIT_CUSTOMER_CONTEXT"),
		"test", // terraformVersion - not available outside provider lifecycle
		"dev",  // providerVersion - matches default in main.go
		provider.DefaultRequestTimeout,
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
