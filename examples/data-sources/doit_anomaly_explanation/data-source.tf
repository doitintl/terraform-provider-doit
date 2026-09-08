# Retrieve the AI-generated explanation for a cost anomaly
data "doit_anomaly_explanation" "example" {
  id = "your-anomaly-id"
}

# ─────────────────────────────────────────────────────────────────────────────
# Deterministic facts about the anomaly (not AI-generated)
# ─────────────────────────────────────────────────────────────────────────────

output "anomaly_explanation_service_name" {
  value = data.doit_anomaly_explanation.example.facts.service_name
}

output "anomaly_explanation_platform" {
  value = data.doit_anomaly_explanation.example.facts.platform
}

output "anomaly_explanation_scope" {
  value = data.doit_anomaly_explanation.example.facts.scope
}

output "anomaly_explanation_severity" {
  value = data.doit_anomaly_explanation.example.facts.severity_level
}

output "anomaly_explanation_cost" {
  value = data.doit_anomaly_explanation.example.facts.cost_of_anomaly
}

# ─────────────────────────────────────────────────────────────────────────────
# AI-generated explanation (non-deterministic; content varies per call)
# ─────────────────────────────────────────────────────────────────────────────

output "anomaly_explanation_text" {
  description = "AI-generated likely-cause explanation. Non-deterministic: may differ on each read."
  value       = data.doit_anomaly_explanation.example.explanation.text
  sensitive   = true
}

output "anomaly_explanation_generated_by" {
  value = data.doit_anomaly_explanation.example.explanation.generated_by
}

output "anomaly_explanation_ai_generated" {
  description = "Always true; identifies the explanation text as AI-generated"
  value       = data.doit_anomaly_explanation.example.explanation.ai_generated
}
