terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "0.25.0"
    }
  }
}

provider "doit" {
  # Configuration options prefer to use environment variables
  # DOIT_API_TOKEN, DOIT_HOST=https://api.doit.com, DOIT_CUSTOMER_CONTEXT
}
