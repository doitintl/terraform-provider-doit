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
