terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.5"
    }
  }
}

# Fetch the current month cloud spend widget using its stable alias
data "doit_widget" "spend" {
  widget_id = "current-month-cloud-spend"
}

# Fetch the current month cloud forecast widget using its stable alias
data "doit_widget" "forecast" {
  widget_id = "current-month-cloud-forecast"
}

# Output formatted metrics
output "spend_metrics" {
  description = "Current month cloud spend metric details"
  value = {
    widget_id    = data.doit_widget.spend.id
    alias        = data.doit_widget.spend.alias
    amount       = data.doit_widget.spend.result.monetary_metric.value.amount
    currency     = data.doit_widget.spend.result.monetary_metric.value.currency
    period_start = data.doit_widget.spend.result.monetary_metric.period.start_time
    period_end   = data.doit_widget.spend.result.monetary_metric.period.end_time
    mom_growth   = data.doit_widget.spend.result.monetary_metric.month_to_month_growth_percentage
    generated_at = data.doit_widget.spend.generate_time
  }
}

# Export widget metrics to CSV format
resource "local_file" "widget_metrics_csv" {
  filename = "${path.module}/widget_metrics.csv"
  content = join("\n", [
    "alias,amount,currency,period_start,period_end,mom_growth_amount,mom_growth_percentage,generated_at",
    join(",", [
      data.doit_widget.spend.alias,
      data.doit_widget.spend.result.monetary_metric.value.amount,
      data.doit_widget.spend.result.monetary_metric.value.currency,
      data.doit_widget.spend.result.monetary_metric.period.start_time,
      data.doit_widget.spend.result.monetary_metric.period.end_time,
      data.doit_widget.spend.result.monetary_metric.month_to_month_growth_amount != null ? data.doit_widget.spend.result.monetary_metric.month_to_month_growth_amount.amount : "",
      data.doit_widget.spend.result.monetary_metric.month_to_month_growth_percentage != null ? tostring(data.doit_widget.spend.result.monetary_metric.month_to_month_growth_percentage) : "",
      data.doit_widget.spend.generate_time,
    ]),
    join(",", [
      data.doit_widget.forecast.alias,
      data.doit_widget.forecast.result.monetary_metric.value.amount,
      data.doit_widget.forecast.result.monetary_metric.value.currency,
      data.doit_widget.forecast.result.monetary_metric.period.start_time,
      data.doit_widget.forecast.result.monetary_metric.period.end_time,
      data.doit_widget.forecast.result.monetary_metric.month_to_month_growth_amount != null ? data.doit_widget.forecast.result.monetary_metric.month_to_month_growth_amount.amount : "",
      data.doit_widget.forecast.result.monetary_metric.month_to_month_growth_percentage != null ? tostring(data.doit_widget.forecast.result.monetary_metric.month_to_month_growth_percentage) : "",
      data.doit_widget.forecast.generate_time,
    ]),
  ])
}
