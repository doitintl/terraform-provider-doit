terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "0.9.0"
    }
  }
}

resource "doit_report" "my-report" {
  name        = "my-report"
  description = "description report"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    advanced_analysis = {
      trending_up   = false
      trending_down = false
      not_trending  = false
      forecast      = false
    }
    "splits" : [
      {
        id : "Up44UNhsdbYW6ACYrWny"
        type : "attribution_group"
        mode : "even"
        include_origin : true
        origin : {
          id : "abLya6iudCLsZYGTpzt0"
          type : "attribution"
        }
        targets : [
          {
            id : "luaETSKhNLhVCbYYTsGF"
            type : "attribution"
          },
          {
            id : "SSa2bJ4vPRZgjmNBaOgW"
            type : "attribution"
          },
          {
            id : "gJ8p6YOQwlr45peIpFW3"
            type : "attribution"
          }
        ]
      }
    ]
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

resource "doit_attribution" "attri" {
  name        = "attritestnewname9"
  description = "attritestdiana8"
  formula     = "A"
  components  = [{ type = "label", key = "iris_location", values = ["us"] }]
}

resource "doit_attribution" "attri2" {
  name        = "attritestnewname10"
  description = "attritestdiana11"
  formula     = "A"
  components  = [{ type = "label", key = "iris_location", values = ["us"] }]
}

resource "doit_attribution" "attri3" {
  name        = "attritestnewname12"
  description = "attritestdiana13"
  formula     = "A"
  components  = [{ type = "label", key = "iris_location", values = ["us"] }]
}

provider "doit" {
}
