terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

variable "service_account_name" {
  type    = string
  default = "terraform-service-account-data-source-example"
}

resource "doit_service_account" "example" {
  name        = var.service_account_name
  description = "Example account for a data source lookup"
  permissions = ["budgetsReadOnly"]
}

data "doit_service_account" "ci" {
  id = doit_service_account.example.id
}

output "service_account_name" {
  value = data.doit_service_account.ci.name
}
