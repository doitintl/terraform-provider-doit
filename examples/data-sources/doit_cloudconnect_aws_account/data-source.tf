# Read the CloudConnect AWS account
data "doit_cloudconnect_aws_account" "example" {
  account_id = "123456789012"
}

# Use the data source to access account details
output "role_arn" {
  value = data.doit_cloudconnect_aws_account.example.role_arn
}

output "supported_features" {
  value = data.doit_cloudconnect_aws_account.example.supported_features
}
