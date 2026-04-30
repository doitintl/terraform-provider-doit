# Use doit_current_user to dynamically get the current user's email
data "doit_current_user" "me" {}

# Define permissions once and reuse across sharing resources.
# Each sharing resource requires exactly one "owner".
locals {
  permissions = [
    { user = data.doit_current_user.me.email, role = "owner" },
    { user = "bob@example.com", role = "editor" },
    { user = "carol@example.com", role = "viewer" },
  ]
}

# ---------------------------------------------------------------------------
# Reports — support all roles (owner, editor, viewer) and public access.
# Public access grants view-only access to all users in the organization.
# ---------------------------------------------------------------------------
resource "doit_report" "example" {
  name = "Shared Cost Report"

  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_range = {
      mode            = "last"
      amount          = 1
      unit            = "month"
      include_current = true
    }
  }
}

resource "doit_sharing" "report" {
  resource_type = "reports"
  resource_id   = doit_report.example.id
  permissions   = local.permissions

  # Optional: Grant view access to all users in the organization.
  # Omitting this attribute (or setting it to null) clears public access.
  public = "viewer"
}

# ---------------------------------------------------------------------------
# Budgets — also have a `collaborators` attribute on the budget resource
# itself. When using doit_sharing with a budget, you should set matching
# collaborators on the budget to avoid plan drift (the sharing API syncs
# permissions to the budget's collaborators server-side).
#
# The budget schema uses "email" instead of "user", so a `for` expression
# converts between the two formats.
# ---------------------------------------------------------------------------
resource "doit_budget" "example" {
  name          = "Shared Budget"
  amount        = 10000
  currency      = "USD"
  type          = "recurring"
  time_interval = "month"
  start_period  = 1735689600000 # 2025-01-01T00:00:00Z in milliseconds

  alerts = [{ percentage = 80 }, { percentage = 100 }]

  # Convert sharing permissions to budget collaborator format (user → email)
  collaborators = [for p in local.permissions : { email = p.user, role = p.role }]

  scopes = [{
    type   = "attribution"
    id     = "attribution"
    mode   = "is"
    values = ["your-attribution-id"]
  }]
}

# Note: Using doit_sharing with a budget is optional when you are already
# defining collaborators directly on the budget resource (as shown above).
# It is primarily useful when you want to manage permissions for budgets
# the same way you manage them for reports, alerts, and allocations.
resource "doit_sharing" "budget" {
  resource_type = "budgets"
  resource_id   = doit_budget.example.id
  permissions   = local.permissions
}

# ---------------------------------------------------------------------------
# Alerts — support all roles (owner, editor, viewer) and public access,
# similar to reports.
# ---------------------------------------------------------------------------
resource "doit_alert" "example" {
  name = "Shared Alert"

  config = {
    metric        = { type = "basic", value = "cost" }
    time_interval = "month"
    condition     = "value"
    operator      = "gt"
    value         = 5000
    currency      = "USD"
  }

  recipients = [data.doit_current_user.me.email]
}

resource "doit_sharing" "alert" {
  resource_type = "alerts"
  resource_id   = doit_alert.example.id
  permissions   = local.permissions
}

# ---------------------------------------------------------------------------
# Allocations — do NOT support the `public` attribute. Setting `public` on
# an allocation will be rejected by the provider at plan time.
# ---------------------------------------------------------------------------
resource "doit_allocation" "example" {
  name        = "Shared Allocation"
  description = "Production environment costs"
  rule = {
    formula = "A"
    components = [{
      mode   = "is"
      type   = "project_label"
      key    = "env"
      values = ["production"]
    }]
  }
}

resource "doit_sharing" "allocation" {
  resource_type = "allocations"
  resource_id   = doit_allocation.example.id
  permissions   = local.permissions

  # Note: `public` is NOT supported for allocations.
  # The provider will reject it at plan time.
}
