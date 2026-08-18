package provider_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// TestAccCustomerResource_CreateErrors asserts that creating a customer without
// importing produces a clear error message.
func TestAccCustomerResource_CreateErrors(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton resource: parallel tests interfere via shared customer settings
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCustomerResourceConfig(""),
				ExpectError: regexp.MustCompile(`Customer Creation Not Supported`),
			},
		},
	})
}

// TestAccCustomerResource_InvalidURLSlug asserts that invalid url_slug values
// are rejected at plan time with descriptive diagnostics.
func TestAccCustomerResource_InvalidURLSlug(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton resource: parallel tests interfere via shared customer settings
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCustomerResourceConfig(`url_slug = "ab"`),
				ExpectError: regexp.MustCompile(`Invalid Customer URL Slug`),
			},
			{
				Config:      testAccCustomerResourceConfig(`url_slug = "my_slug"`),
				ExpectError: regexp.MustCompile(`Invalid Customer URL Slug`),
			},
			{
				Config:      testAccCustomerResourceConfig(`url_slug = "-abc"`),
				ExpectError: regexp.MustCompile(`Invalid Customer URL Slug`),
			},
		},
	})
}

// TestAccCustomerResource_Import tests importing the customer resource and verifying drift.
func TestAccCustomerResource_Import(t *testing.T) {
	customer := testAccGetCustomer(t)

	// Singleton resource — cannot run in parallel with other customer tests.
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton resource: parallel tests interfere via shared customer settings
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Import and persist state
			{
				Config:             testAccCustomerResourceConfig(""),
				ResourceName:       "doit_customer.test",
				ImportState:        true,
				ImportStateId:      customer.Id,
				ImportStatePersist: true,
				ImportStateVerify:  false,
			},
			// Step 2: Verify fields are populated after import
			{
				Config: testAccCustomerResourceConfig(""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_customer.test", "id", customer.Id),
					resource.TestCheckResourceAttrSet("doit_customer.test", "name"),
					resource.TestCheckResourceAttrSet("doit_customer.test", "primary_domain"),
				),
			},
			// Step 3: Drift verification - re-apply should produce an empty plan
			{
				Config: testAccCustomerResourceConfig(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccCustomerResource_Lifecycle tests importing, updating, clearing, drift checks,
// and restoring customer general settings.
func TestAccCustomerResource_Lifecycle(t *testing.T) {
	original := testAccGetCustomer(t)
	t.Cleanup(func() {
		testAccRestoreCustomer(t, original)
	})

	testEmail := "tf-acc-test@example.com"
	testDomain := "example.com"

	updateConfig := testAccCustomerResourceConfig(fmt.Sprintf(`
  contact = {
    emails = [%q]
  }
  settings = {
    allowed_invite_domains = [%q]
  }
`, testEmail, testDomain))

	omittedConfig := testAccCustomerResourceConfig("")

	clearedConfig := testAccCustomerResourceConfig(`
  url_slug = ""
  contact = {
    emails = []
  }
  settings = {
    allowed_invite_domains = []
  }
`)

	restoredConfig := testAccCustomerResourceRestoreConfig(original)

	// Singleton resource — cannot run in parallel with other customer tests.
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton resource: parallel tests interfere via shared customer settings
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Import and persist state
			{
				Config:             testAccCustomerResourceConfig(""),
				ResourceName:       "doit_customer.test",
				ImportState:        true,
				ImportStateId:      original.Id,
				ImportStatePersist: true,
				ImportStateVerify:  false,
			},
			// Step 2: Update contact emails and allowed_invite_domains
			{
				Config: updateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_customer.test", "id", original.Id),
					resource.TestCheckResourceAttr("doit_customer.test", "contact.emails.0", testEmail),
					resource.TestCheckResourceAttr("doit_customer.test", "settings.allowed_invite_domains.0", testDomain),
				),
			},
			// Step 3: Drift check after update
			{
				Config: updateConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 4: Clear by omitting contact and settings blocks (Category A plan modifiers)
			{
				Config: omittedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_customer.test", "id", original.Id),
					resource.TestCheckResourceAttr("doit_customer.test", "contact.emails.#", "0"),
					resource.TestCheckResourceAttr("doit_customer.test", "settings.allowed_invite_domains.#", "0"),
				),
			},
			// Step 5: Drift check after clearing via omission
			{
				Config: omittedConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 6: Explicit empty values for contact emails, allowed_invite_domains, and url_slug
			{
				Config: clearedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_customer.test", "id", original.Id),
					resource.TestCheckResourceAttr("doit_customer.test", "url_slug", ""),
					resource.TestCheckResourceAttr("doit_customer.test", "contact.emails.#", "0"),
					resource.TestCheckResourceAttr("doit_customer.test", "settings.allowed_invite_domains.#", "0"),
				),
			},
			// Step 7: Drift check after explicit clear
			{
				Config: clearedConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 8: Restore initial settings via HCL
			{
				Config: restoredConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_customer.test", "id", original.Id),
				),
			},
			// Step 9: Final drift check on restored config
			{
				Config: restoredConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// --- Test helper functions ---

func testAccCustomerResourceConfig(extraAttrs string) string {
	if extraAttrs != "" {
		return fmt.Sprintf(`
resource "doit_customer" "test" {
  %s
}
`, strings.TrimSpace(extraAttrs))
	}
	return `
resource "doit_customer" "test" {
}
`
}

func testAccCustomerResourceRestoreConfig(original *models.Customer) string {
	var parts []string
	if original.UrlSlug != nil && *original.UrlSlug != "" {
		parts = append(parts, fmt.Sprintf("url_slug = %q", *original.UrlSlug))
	}
	if original.Settings != nil {
		var settingsParts []string
		if original.Settings.Currency != nil {
			settingsParts = append(settingsParts, fmt.Sprintf("currency = %q", string(*original.Settings.Currency)))
		}
		if original.Settings.AllowedInviteDomains != nil {
			var quotedDomains []string
			for _, d := range *original.Settings.AllowedInviteDomains {
				quotedDomains = append(quotedDomains, fmt.Sprintf("%q", d))
			}
			settingsParts = append(settingsParts, fmt.Sprintf("allowed_invite_domains = [%s]", strings.Join(quotedDomains, ", ")))
		}
		if len(settingsParts) > 0 {
			parts = append(parts, fmt.Sprintf("settings = {\n    %s\n  }", strings.Join(settingsParts, "\n    ")))
		}
	}
	if original.Contact != nil && original.Contact.Emails != nil {
		var quotedEmails []string
		for _, e := range *original.Contact.Emails {
			quotedEmails = append(quotedEmails, fmt.Sprintf("%q", string(e)))
		}
		parts = append(parts, fmt.Sprintf("contact = {\n    emails = [%s]\n  }", strings.Join(quotedEmails, ", ")))
	}

	return testAccCustomerResourceConfig(strings.Join(parts, "\n  "))
}

func testAccGetCustomer(t *testing.T) *models.Customer {
	t.Helper()
	skipIfNoAcc(t)

	client := getAPIClient(t)
	resp, err := client.GetCustomerWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to get customer: %v", err)
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		t.Fatalf("Failed to get customer: status %d, body: %s", resp.StatusCode(), string(resp.Body))
	}
	return resp.JSON200
}

func testAccRestoreCustomer(t *testing.T, original *models.Customer) {
	t.Helper()

	client := getAPIClient(t)
	req := models.CustomerUpdate{}

	if original.UrlSlug != nil {
		req.UrlSlug = original.UrlSlug
	} else {
		empty := ""
		req.UrlSlug = &empty
	}

	if original.Settings != nil {
		req.Settings = &models.CustomerSettings{
			Currency:             original.Settings.Currency,
			AllowedInviteDomains: original.Settings.AllowedInviteDomains,
		}
		if req.Settings.AllowedInviteDomains == nil {
			empty := []string{}
			req.Settings.AllowedInviteDomains = &empty
		}
	}

	if original.Contact != nil && original.Contact.Emails != nil {
		req.Contact = &models.CustomerContact{
			Emails: original.Contact.Emails,
		}
	} else {
		empty := []openapi_types.Email{}
		req.Contact = &models.CustomerContact{
			Emails: &empty,
		}
	}

	resp, err := client.UpdateCustomerWithApplicationMergePatchPlusJSONBodyWithResponse(context.Background(), nil, req)
	if err != nil {
		t.Logf("Failed to restore customer settings during cleanup: %v", err)
		return
	}
	if resp.StatusCode() != 200 {
		t.Logf("Failed to restore customer settings during cleanup: status %d, body: %s", resp.StatusCode(), string(resp.Body))
	}
}
