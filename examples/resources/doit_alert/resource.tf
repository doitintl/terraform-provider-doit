# Create an alert that triggers when costs exceed $1000 per month
resource "doit_alert" "cost_alert" {
  name = "Monthly Cost Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
  recipients = ["finance@company.com", "ops@company.com"]
}

# Alert with scope filters
resource "doit_alert" "aws_cost_alert" {
  name = "AWS Cost Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "day"
    value         = 100
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "fixed"
        id     = "cloud_provider"
        mode   = "is"
        values = ["amazon-web-services"]
      }
    ]
  }
  recipients = ["ops@company.com"]
}
