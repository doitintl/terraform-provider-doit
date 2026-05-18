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

# Look up a specific user by email
data "doit_users" "specific" {
  email = "user@example.com"
}

output "user_id" {
  value = one(data.doit_users.specific.users[*].id)
}
