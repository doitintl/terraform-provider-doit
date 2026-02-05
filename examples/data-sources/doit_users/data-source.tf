# List all users in the organization
data "doit_users" "all" {}

# Output user count
output "user_count" {
  value = data.doit_users.all.row_count
}

# Get list of user emails
output "user_emails" {
  value = [for u in data.doit_users.all.users : u.email]
}
