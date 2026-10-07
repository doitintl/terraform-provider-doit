# Get information about the current authenticated user
data "doit_current_user" "me" {}

# Output the current user's email
output "my_email" {
  value = data.doit_current_user.me.email
}

output "my_domain" {
  value = data.doit_current_user.me.domain
}

output "my_permissions" {
  value = data.doit_current_user.me.permissions
}

# Fail early with a readable message when the identity running Terraform
# lacks a permission the configuration needs
data "doit_current_user" "caller" {
  lifecycle {
    postcondition {
      condition     = alltrue([for p in ["UsersManager", "ServiceAccountManager"] : contains(self.permissions, p)])
      error_message = "The identity running Terraform needs the UsersManager and ServiceAccountManager permissions."
    }
  }
}
