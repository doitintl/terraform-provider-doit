terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

variable "service_account_name" {
  type    = string
  default = "terraform-service-account-tokens-data-source-example"
}

resource "doit_service_account" "example" {
  name = var.service_account_name
}

resource "doit_service_account_token" "example" {
  service_account_id = doit_service_account.example.id
  name               = "example"
}

# List all API tokens of a service account (at most 10, not paginated).
# The access tokens themselves are never returned by the data source.
data "doit_service_account_tokens" "all" {
  service_account_id = doit_service_account.example.id

  depends_on = [doit_service_account_token.example]
}

output "token_names" {
  value = [for token in data.doit_service_account_tokens.all.items : token.name]
}

output "token_expiry" {
  value = { for token in data.doit_service_account_tokens.all.items : token.name => token.expires_time }
}
