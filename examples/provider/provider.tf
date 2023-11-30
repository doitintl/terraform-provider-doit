terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "0.2.1"
    }
  }
}

provider "doit" {
  # Configuration options prefer to use environment variables
  # DOIT_API_TOKEN, DOIT_HOST=https://api.doit.com and DOIT_CUSTOMER_CONTEXT
}
