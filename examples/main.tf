terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "0.10.0"
    }
  }
}

resource "doit_report" "my-report_january" {
  name        = "my-report"
  description = "description report"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    sort_groups = "asc"
    sort_dimensions = "asc"
    advanced_analysis = {
      trending_up   = true
      trending_down = false
      not_trending  = false
      forecast      = false
    }
    dimensions = [
      {
        id   = "year"
        type = "datetime"
      },
      {
        id   = "month"
        type = "datetime"
      }
    ]
    time_range = {
      mode            = "last"
      amount          = 7
      include_current = true
      unit            = "day"
    }
    include_promotional_credits = false
    filters = [
      {
        id   = "attribution"
        type = "attribution"
        values = [
          "1CE699ZdwN5CRBw0tInY"
        ]
      }
    ]
    group = [
      {
        id   = "BSQZmvX6hvuKGPDHX7R3"
        type = "attribution_group"
        limit = {
          value = 3
          sort  = "a_to_z"
          metric = {
            type  = "basic"
            value = "cost"
          }
        }
      },
      {
        id   = "cloud_provider"
        type = "fixed"
        limit = {
          value = 10
          sort  = "a_to_z"
          metric = {
            type  = "basic"
            value = "cost"
          }
        }
      }
    ]
  }
}

provider "doit" {
}
