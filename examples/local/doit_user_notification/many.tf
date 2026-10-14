resource "doit_user_notification" "many" {
  for_each = var.users

  user_id = each.value
  name    = "CMP-54313 local ${each.key}"

  settings = {
    cost_anomalies = {
      min_severity = "warning"
      min_excess_cost = {
        amount   = "100.00"
        currency = "USD"
      }
      allocations = {
        mode = "include"
        ids  = var.allocation_ids
      }
      repeat = {
        every_hours = 24
      }
    }
  }
}
