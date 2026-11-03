# Retrieve all Composer recipes
data "doit_composer_recipes" "all" {}

# Filter recipes with active results and sort by severity
data "doit_composer_recipes" "active" {
  filter     = "has_active_results:true"
  sort_by    = "severity"
  sort_order = "desc"
}

output "total_recipes" {
  value = data.doit_composer_recipes.all.row_count
}

output "active_recipe_summaries" {
  value = [for r in data.doit_composer_recipes.active.items : {
    id                   = r.id
    title                = r.title
    provider             = r.provider
    severity             = r.severity
    active_results_count = r.active_results_count
  }]
}
