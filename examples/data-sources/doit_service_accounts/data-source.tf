terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

data "doit_service_accounts" "all" {}

output "service_account_ids" {
  value = [for account in data.doit_service_accounts.all.items : account.id]
}
