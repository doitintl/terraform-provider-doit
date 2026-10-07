terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

variable "service_account_name" {
  type    = string
  default = "terraform-service-account-token-data-source-example"
}

resource "doit_service_account" "example" {
  name = var.service_account_name
}

resource "doit_service_account_token" "example" {
  service_account_id = doit_service_account.example.id
  name               = "example"
}

# Look up a single API token of a service account by its ID.
# The access token itself is never returned by the data source.
data "doit_service_account_token" "example" {
  service_account_id = doit_service_account.example.id
  id                 = doit_service_account_token.example.id
}

output "token_state" {
  value = data.doit_service_account_token.example.state
}

output "token_expires_time" {
  value = data.doit_service_account_token.example.expires_time
}
