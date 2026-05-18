# Retrieve all folders
data "doit_folders" "all" {}

# Output folder count
output "total_folders" {
  value = data.doit_folders.all.row_count
}

# Output folder names
output "folder_names" {
  value = [for f in data.doit_folders.all.folders : f.name]
}

# Retrieve the first page of folders
data "doit_folders" "first_page" {
  max_results = "5"
}
