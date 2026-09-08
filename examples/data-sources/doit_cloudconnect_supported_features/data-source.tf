data "doit_cloudconnect_supported_features" "aws" {
  account_id = "123456789012"
}

output "aws_feature_permissions" {
  value = data.doit_cloudconnect_supported_features.aws.supported_features
}
