# Create a top-level folder for organizing reports
resource "doit_folder" "analytics" {
  name        = "Analytics"
  description = "Cloud Analytics reports and dashboards"
}

# Create a subfolder nested under the analytics folder
resource "doit_folder" "cost_reports" {
  name             = "Cost Reports"
  description      = "Monthly and quarterly cost breakdowns"
  parent_folder_id = doit_folder.analytics.id
}

# Create a folder with only required fields (defaults to root)
resource "doit_folder" "archive" {
  name = "Archive"
}
