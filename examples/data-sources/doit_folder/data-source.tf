# Retrieve a folder by its ID
data "doit_folder" "example" {
  id = "your-folder-id"
}

# Output folder details
output "folder_name" {
  value = data.doit_folder.example.name
}

output "folder_parent" {
  value = data.doit_folder.example.parent_folder_id
}

# Use a folder ID when creating a report in that folder
resource "doit_report" "example" {
  name      = "Cost by Service"
  folder_id = data.doit_folder.example.id

  config {
    metric {
      type  = "basic"
      value = "cost"
    }
    time_range {
      mode           = "last"
      amount         = 1
      unit           = "month"
      include_current = true
    }
  }
}
