terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
  }
}

# The user behind the API token must be a collaborator of the connection,
# otherwise the API denies access to it right after creating it. Declare that
# user as the owner; a connection has exactly one owner.

# GCP connection scoped to a single project.
# The project must be known to your DoiT customer.
resource "doit_cloudflow_connection" "gcp" {
  name        = "Production project"
  description = "Lets CloudFlow workflows act on the production project"

  collaborators = [
    { email = "owner@example.com", role = "owner" },
    { email = "engineer@example.com", role = "editor" },
  ]

  gcp_config = {
    level                = "project"
    project_id           = "my-production-project"
    service_account_name = "doit-cloudflow"
    predefined_roles     = ["roles/viewer"]

    custom_role = {
      role_id     = "doitCloudflowOperator"
      permissions = ["compute.instances.list", "compute.instances.get"]
    }
  }
}

# AWS connection covering two accounts, with a managed policy and custom actions.
resource "doit_cloudflow_connection" "aws" {
  name = "Production accounts"

  collaborators = [
    { email = "owner@example.com", role = "owner" },
  ]

  aws_config = {
    role_name = "doit-cloudflow"

    context = [
      { account_id = "123456789012", regions = ["us-east-1", "eu-west-1"] },
      { account_id = "210987654321" },
    ]

    permissions = {
      aws_managed_policies = ["arn:aws:iam::aws:policy/ReadOnlyAccess"]
      actions              = [{ value = "s3:ListBucket" }, { value = "ec2:DescribeInstances" }]
    }
  }
}

# A disabled connection is kept but cannot be used by flows.
resource "doit_cloudflow_connection" "disabled" {
  name    = "Paused connection"
  enabled = false

  collaborators = [
    { email = "owner@example.com", role = "owner" },
  ]

  aws_config = {
    role_name = "doit-cloudflow-paused"
    context   = [{ account_id = "111111111111" }]
  }
}
