resource "doit_user_notification" "test" {
  user_id = var.user_id
  name    = "CMP-54313 local test"

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
