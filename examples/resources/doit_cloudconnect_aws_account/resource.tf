# Connect an AWS account with just the IAM role
resource "doit_cloudconnect_aws_account" "basic" {
  account_id       = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/DoiTRole"
  enabled_features = []
}

# Connect an AWS account with real-time anomaly detection (S3 bucket)
resource "doit_cloudconnect_aws_account" "with_realtime" {
  account_id       = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/DoiTRole"
  enabled_features = ["real-time-data"]
  s3bucket         = "my-cloudtrail-bucket"
  s3bucket_region  = "us-east-1"
}
