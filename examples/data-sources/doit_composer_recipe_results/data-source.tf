# Retrieve all results for a Composer recipe
data "doit_composer_recipe_results" "all" {
  recipe_id = 569
}

# Filter for active results only, sorted by detection time
data "doit_composer_recipe_results" "active" {
  recipe_id  = 569
  filter     = "status:active"
  sort_by    = "detection_time"
  sort_order = "desc"
}

output "result_count" {
  value = data.doit_composer_recipe_results.all.row_count
}

output "active_findings" {
  value = [for res in data.doit_composer_recipe_results.active.items : {
    id                   = res.id
    account_id           = res.account_id
    issued_resource_name = res.issued_resource_name
    provider             = res.provider
    region               = res.region
  }]
}
