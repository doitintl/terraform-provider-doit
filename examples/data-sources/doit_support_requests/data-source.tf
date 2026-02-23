# List all support requests
data "doit_support_requests" "all" {
}

# Filter support requests by severity
data "doit_support_requests" "high_severity" {
  filter = "severity:high"
}

output "ticket_count" {
  value = data.doit_support_requests.all.row_count
}

# ─────────────────────────────────────────────────────────────────────────────
# Combining data sources: filter and enrich support requests
# ─────────────────────────────────────────────────────────────────────────────

# Use doit_users to find a specific user's email, then filter support requests
data "doit_users" "all" {}

# Filter support requests by a specific user from the users data source
# The filter syntax for requester is "requester:<email>"
output "tickets_by_user" {
  description = "Support tickets grouped by requester from the users data source"
  value = {
    for requester, tickets in { for t in data.doit_support_requests.all.tickets : t.requester => t... } : requester => [
      for t in tickets : {
        id       = t.id
        subject  = t.subject
        severity = t.severity
        status   = t.status
        platform = t.platform
      }
    ] if contains([for u in data.doit_users.all.users : u.email], requester)
  }
}

# Use doit_platforms to discover valid platform names for support request filtering
data "doit_platforms" "all" {}

output "tickets_by_platform" {
  description = "Support tickets grouped by platform"
  value = {
    for platform, tickets in { for t in data.doit_support_requests.all.tickets : t.platform => t... } : platform => [
      for t in tickets : {
        id       = t.id
        subject  = t.subject
        severity = t.severity
      }
    ] if contains([for p in data.doit_platforms.all.platforms : p.display_name], platform)
  }
}
