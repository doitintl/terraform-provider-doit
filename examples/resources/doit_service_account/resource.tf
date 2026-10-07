# Create a basic service account
resource "doit_service_account" "ci_cd" {
  name = "ci-cd-pipeline"
}

# Create a service account with description and permissions
resource "doit_service_account" "analytics_exporter" {
  name        = "analytics-exporter"
  description = "Service account used by the daily analytics export cron job"
  permissions = [
    "cloudAnalyticsReadOnly",
    "budgetsManager",
  ]
}
