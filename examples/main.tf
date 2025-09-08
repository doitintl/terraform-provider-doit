terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "0.25.0"
    }
  }
}



resource "doit_budget" "my_budget" {
  name        = "test budget terraform"
  description = "hellogo test2"
  alerts = [
    {
      percentage = 50
    },
    {
      percentage = 85,
    },
    {
      percentage = 100,
    }
  ]
  recipients = [
    "test@doit.com"
  ]
  collaborators = [
    {
      "email" : "test@doit.com",
      "role" : "owner"
    },
  ]
  scope = [
    "Evct3J0DYcyXIVuAXORd"
  ]
  amount            = 200
  currency          = "AUD"
  growth_per_period = 10
  time_interval     = "month"
  type              = "recurring"
  use_prev_spend    = false
}

resource "doit_attribution" "attri" {
  name        = "attritestnewname"
  description = "attritestdesc"
  formula     = "A"
  components  = [{ type = "system_label", key = "aws/region_code", values = ["us-east-1"] }]
}

resource "doit_attribution_group" "attributeGroup" {
  name         = "attritestnewgroup"
  description  = "attritestgroup-new-new"
  attributions = [doit_attribution.attri.id]
}

resource "doit_report" "my-report_january" {
  name        = "my-report"
  description = "description report"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    sort_groups     = "asc"
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
