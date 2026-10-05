# Invite a new user with minimal configuration
resource "doit_user" "basic" {
  email = "newuser@example.com"
}

# Invite a user with full profile
resource "doit_user" "full" {
  email           = "jane.doe@example.com"
  first_name      = "Jane"
  last_name       = "Doe"
  job_title       = "Software / Ops Engineer"
  role_id         = "role-id-here"
  organization_id = "org-id-here"
  phone           = "+1"
  phone_extension = "5551234567"
  language        = "en"
}

# Invite a team from a map and assign everyone a custom role
resource "doit_role" "analyst" {
  name = "Analyst"
}

locals {
  analysts = {
    "ana@example.com"  = { first_name = "Ana", last_name = "Lopez" }
    "ravi@example.com" = { first_name = "Ravi", last_name = "Patel" }
  }
}

resource "doit_user" "analyst" {
  for_each = local.analysts

  email      = each.key
  first_name = each.value.first_name
  last_name  = each.value.last_name
  job_title  = "Finance / Accounting"
  role_id    = doit_role.analyst.id
}
