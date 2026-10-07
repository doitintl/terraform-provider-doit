# Use doit_current_user to dynamically get the current user's email
data "doit_current_user" "me" {}

# Create a timestamp for the start period
resource "time_static" "now" {
  rfc3339 = "2025-12-01T00:00:00Z"
}

resource "doit_budget" "this" {
  name          = "My Budget"
  currency      = "AUD"
  type          = "recurring"
  amount        = 100
  time_interval = "month"
  start_period  = time_static.now.unix * 1000 # This is a UNIX timestamp in milliseconds
  # Instead of using a separate resource to get the unix timestamp, you can also use:
  # provider::time::rfc3339_parse("2025-12-01T00:00:00Z") * 1000
  alerts = [
    { percentage = 50 },
    { percentage = 80 },
    { percentage = 100 }
  ]
  collaborators = [
    {
      "email" : data.doit_current_user.me.email,
      "role" : "owner"
    },
  ]
  scopes = [
    {
      type   = "allocation_rule"
      id     = "allocation_rule"
      mode   = "is"
      values = ["ydDBFKVuz9kGlFDex8cN"]
    }
  ]
}

# ─────────────────────────────────────────────────────────────────────────────
# Discovering valid scope and collaborator values using data sources
# ─────────────────────────────────────────────────────────────────────────────
# Budget scopes use the same id/type/mode/values structure as report filters.
# Use doit_allocations, doit_dimensions, and doit_users to populate these dynamically.

# Use doit_allocations to scope a budget to an existing allocation
data "doit_allocations" "all" {}

resource "doit_budget" "scoped_to_allocation" {
  name          = "Dev Environment Budget"
  currency      = "USD"
  type          = "recurring"
  amount        = 5000
  time_interval = "month"
  start_period  = time_static.now.unix * 1000
  alerts = [
    { percentage = 80 },
    { percentage = 100 }
  ]
  # Scope the budget to the first allocation from the data source
  scopes = [
    {
      type   = "allocation_rule"
      id     = "allocation_rule"
      mode   = "is"
      values = [for a in data.doit_allocations.all.allocations : a.id]
    }
  ]
}

# Use doit_users to populate budget collaborator emails
data "doit_users" "all" {}

# Use doit_dimensions to look up correct dimension types for scope fields
data "doit_dimensions" "all" {}

locals {
  dimension_types = { for id, types in {
    for d in data.doit_dimensions.all.dimensions : d.id => d.type...
  } : id => types[0] }
}

resource "doit_budget" "with_collaborators" {
  name          = "Team Budget"
  currency      = "USD"
  type          = "recurring"
  amount        = 10000
  time_interval = "month"
  start_period  = time_static.now.unix * 1000
  alerts = [
    { percentage = 50 },
    { percentage = 80 },
    { percentage = 100 }
  ]
  # Use the dimension lookup to scope this budget
  scopes = [
    {
      id     = "cloud_provider"
      type   = local.dimension_types["cloud_provider"]
      mode   = "is"
      values = ["google-cloud"]
    }
  ]
  # Use user emails from the users data source as collaborators
  collaborators = [for u in data.doit_users.all.users : {
    email = u.email
    role  = "viewer"
  }]
  # Notify all users when thresholds are reached
  recipients = [for u in data.doit_users.all.users : u.email]
}

# ─────────────────────────────────────────────────────────────────────────────
# Using doit_dimension (singular) to discover valid scope values
# ─────────────────────────────────────────────────────────────────────────────
# While doit_dimensions (plural) helps look up dimension types,
# doit_dimension (singular) retrieves the *values* for a specific dimension.
# This lets you dynamically populate scope values instead of hardcoding them.

# Look up valid cloud_provider values from the API
data "doit_dimension" "cloud_provider" {
  type = "fixed"
  id   = "cloud_provider"
}

resource "doit_budget" "dynamic_scope" {
  name          = "Dynamic Cloud Budget"
  currency      = "USD"
  type          = "recurring"
  amount        = 50000
  time_interval = "month"
  start_period  = time_static.now.unix * 1000
  alerts = [
    { percentage = 80 },
    { percentage = 100 }
  ]
  # Scope values discovered dynamically from the API
  scopes = [
    {
      id     = "cloud_provider"
      type   = "fixed"
      mode   = "is"
      values = [for v in data.doit_dimension.cloud_provider.values : v.value]
    }
  ]
}

# ─────────────────────────────────────────────────────────────────────────────
# One budget per team, with email and Slack notifications
# ─────────────────────────────────────────────────────────────────────────────
# Discover team allocations by naming convention (see the allocation factory
# example on doit_allocation) and create a budget for each one.

data "doit_allocations" "teams" {
  name_contains = "Team: "
}

locals {
  team_budgets = {
    "Team: payments" = 12000
    "Team: search"   = 8000
    "Team: platform" = 20000
  }
}

resource "doit_budget" "team" {
  for_each = {
    for a in data.doit_allocations.teams.allocations : a.name => a
    if contains(keys(local.team_budgets), a.name)
  }

  name          = each.key
  type          = "recurring"
  time_interval = "month"
  currency      = "USD"
  amount        = local.team_budgets[each.key]
  start_period  = time_static.now.unix * 1000

  alerts = [{ percentage = 80 }, { percentage = 100 }]

  scopes = [{
    type   = "allocation_rule"
    id     = "allocation_rule"
    mode   = "is"
    values = [each.value.id]
  }]

  collaborators = [{ email = data.doit_current_user.me.email, role = "owner" }]
  recipients    = [data.doit_current_user.me.email]

  # Budgets require the Slack channel ID, your customer ID and the connected
  # workspace name for each Slack destination.
  recipients_slack_channels = [{
    id          = "C0123456789"
    customer_id = "your-customer-id"
    workspace   = "your-slack-workspace"
  }]
}
