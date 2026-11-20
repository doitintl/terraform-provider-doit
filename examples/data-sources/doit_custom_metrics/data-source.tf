# List all custom metrics
data "doit_custom_metrics" "all" {}

# Filter by name substring (case-insensitive)
data "doit_custom_metrics" "share" {
  name_contains = "share"
}

# Filter by owner. Percent-encode ":", ",", "|" and spaces inside filter values;
# urlencode() does this for an email address.
data "doit_custom_metrics" "mine" {
  filter = "owner:${urlencode("finops@example.com")}"
}

# Newest first is the default; sort by name instead
data "doit_custom_metrics" "by_name" {
  sort_by    = "name"
  sort_order = "asc"
}

# Output the number of custom metrics
output "total_custom_metrics" {
  value = data.doit_custom_metrics.all.row_count
}

# Map metric names to IDs
output "custom_metric_ids" {
  value = {
    for m in data.doit_custom_metrics.all.items : m.name => m.id
  }
}
