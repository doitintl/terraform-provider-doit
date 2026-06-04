# Get diagram stats for the last 30 days.
data "doit_cloud_diagrams_stats" "example" {
  start = timeadd(plantimestamp(), "-720h")
  end   = plantimestamp()
}

# Output diagrams with changes.
output "diagrams_with_changes" {
  value = [
    for stat in data.doit_cloud_diagrams_stats.example.cloud_diagrams_stats :
    stat if length(stat.changes) > 0
  ]
}
