# Manage sharing permissions for a Cloud Analytics report
resource "doit_resource_sharing" "example" {
  resource_type = "reports"
  resource_id   = doit_report.example.id

  permissions = [
    {
      user = "alice@example.com"
      role = "owner"
    },
    {
      user = "bob@example.com"
      role = "editor"
    },
    {
      user = "carol@example.com"
      role = "viewer"
    }
  ]

  # Optional: Grant view access to all users in the organization.
  # Omitting this attribute (or setting it to null) clears public access.
  public = "viewer"
}
