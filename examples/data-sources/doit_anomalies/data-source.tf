# Retrieve all anomalies
data "doit_anomalies" "all" {}

# Filter by severity
data "doit_anomalies" "critical" {
  filter = "severityLevel:critical"
}

# Output anomaly details
output "total_anomalies" {
  value = data.doit_anomalies.all.row_count
}

output "anomaly_summary" {
  value = [for a in data.doit_anomalies.all.anomalies : {
    id          = a.id
    service     = a.service_name
    cost_impact = a.cost_of_anomaly
    severity    = a.severity_level
    status      = a.status
  }]
}

# ─────────────────────────────────────────────────────────────────────────────
# Combining data sources: enrich anomalies with product and platform info
# ─────────────────────────────────────────────────────────────────────────────

# Use doit_platforms to discover available platforms
data "doit_platforms" "all" {}

# Group anomalies by platform
output "anomalies_by_platform" {
  description = "Cost anomalies grouped by cloud platform"
  value = {
    for platform, anomalies in { for a in data.doit_anomalies.all.anomalies : a.platform => a... } : platform => [
      for a in anomalies : {
        id          = a.id
        service     = a.service_name
        cost_impact = a.cost_of_anomaly
        severity    = a.severity_level
      }
    ] if contains([for p in data.doit_platforms.all.platforms : p.display_name], platform)
  }
}

# Use doit_products to cross-reference anomaly services with known products
data "doit_products" "all" {}

output "anomalies_with_product_match" {
  description = "Anomalies enriched with whether their service matches a known product"
  value = [for a in data.doit_anomalies.all.anomalies : {
    id            = a.id
    service       = a.service_name
    platform      = a.platform
    cost_impact   = a.cost_of_anomaly
    severity      = a.severity_level
    known_product = contains([for p in data.doit_products.all.products : p.display_name], a.service_name)
  }]
}
