# Get information about the current authenticated user
data "doit_current_user" "me" {}

# Output the current user's email
output "my_email" {
  value = data.doit_current_user.me.email
}

output "my_domain" {
  value = data.doit_current_user.me.domain
}
