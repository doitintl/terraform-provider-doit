# Manage sharing permissions for a Cloud Analytics report
resource "doit_report" "example" {
  name = "Shared Cost Report"

  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_range = {
      mode           = "last"
      amount         = 1
      unit           = "month"
      include_current = true
    }
  }
}

resource "doit_sharing" "example" {
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
