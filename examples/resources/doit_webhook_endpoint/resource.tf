# Create a basic webhook endpoint that subscribes to all events
resource "doit_webhook_endpoint" "all_events" {
  name = "All Events Webhook"
  url  = "https://hooks.example.com/doit/all"
}

# Create a webhook endpoint with a description and specific event subscriptions
resource "doit_webhook_endpoint" "cost_alerts" {
  name        = "Cost Alert Notifications"
  url         = "https://hooks.example.com/doit/costs"
  description = "Receives cost-related alerts and budget notifications"
  events = [
    "alert.triggered",
    "alert.resolved",
    "budget.exceeded",
  ]
}

# Create a webhook endpoint for anomaly detection
resource "doit_webhook_endpoint" "anomaly_detection" {
  name        = "Anomaly Detection Webhook"
  url         = "https://hooks.example.com/doit/anomalies"
  description = "Receives anomaly detection and resolution events"
  events = [
    "anomaly.detected",
    "anomaly.resolved",
  ]
}
