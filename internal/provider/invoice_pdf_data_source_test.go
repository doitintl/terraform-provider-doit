package provider_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestInvoicePdfDataSource_Unit_UnknownID(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "doit" {
  host      = "http://127.0.0.1:1"
  api_token = "test-token"
}

resource "terraform_data" "invoice_id" {
  input = "INV-12345"
}

data "doit_invoice_pdf" "test" {
  id = terraform_data.invoice_id.output
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccInvoicePdfDataSource_Basic(t *testing.T) {
	invoiceID := os.Getenv("TEST_INVOICE_ID")
	if invoiceID == "" {
		t.Skip("TEST_INVOICE_ID environment variable not set")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, invoiceID+".pdf")
	config := testAccInvoicePdfDataSourceConfig(invoiceID) + fmt.Sprintf(`
resource "local_sensitive_file" "invoice_pdf" {
  filename       = "%s/${data.doit_invoice_pdf.test.filename}"
  content_base64 = data.doit_invoice_pdf.test.content_base64
}
`, filepath.ToSlash(tempDir))

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"local": {Source: "hashicorp/local", VersionConstraint: "~> 2.5"},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_invoice_pdf.test", "id", invoiceID),
					resource.TestCheckResourceAttr("data.doit_invoice_pdf.test", "filename", invoiceID+".pdf"),
					resource.TestCheckResourceAttrWith("data.doit_invoice_pdf.test", "content_base64", func(value string) error {
						if value == "" {
							return fmt.Errorf("expected non-empty content_base64")
						}
						decoded, err := base64.StdEncoding.DecodeString(value)
						if err != nil {
							return fmt.Errorf("content_base64 is not valid base64: %w", err)
						}
						if !bytes.HasPrefix(decoded, []byte("%PDF-")) {
							return fmt.Errorf("decoded content does not start with %%PDF- header")
						}
						return nil
					}),
					func(_ *terraform.State) error {
						content, err := os.ReadFile(outputPath)
						if err != nil {
							return err
						}
						if !bytes.HasPrefix(content, []byte("%PDF-")) {
							return fmt.Errorf("downloaded file does not have a PDF header")
						}
						return nil
					},
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccInvoicePdfDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccInvoicePdfDataSourceConfig("NONEXISTENT-INVOICE-ID"),
				ExpectError: regexp.MustCompile(`Invoice PDF with ID NONEXISTENT-INVOICE-ID not found`),
			},
		},
	})
}

func testAccInvoicePdfDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_invoice_pdf" "test" {
  id = "%s"
}
`, id)
}
