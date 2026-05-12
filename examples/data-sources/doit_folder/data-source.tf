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
