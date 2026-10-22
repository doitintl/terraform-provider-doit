# Download an invoice PDF by its invoice number
data "doit_invoice_pdf" "example" {
  id = "your-invoice-number"
}

# Save the decoded PDF to disk using the hashicorp/local provider
resource "local_file" "invoice_pdf" {
  filename       = "${path.module}/invoice-${data.doit_invoice_pdf.example.id}.pdf"
  content_base64 = data.doit_invoice_pdf.example.content_base64
}
