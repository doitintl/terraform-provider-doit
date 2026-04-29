# Manage custom insights through the DoiT Insights API.
# Insights are organized by source and key, allowing external tools to push
# optimization findings (e.g. cost savings, security risks) into the DoiT console.

# Basic cost savings insight with a single resource result
resource "doit_insight" "unused_instance" {
  key               = "unused-ec2-instances"
  title             = "Unused EC2 Instances"
  short_description = "EC2 instances with consistently low CPU utilization"
  cloud_provider    = "aws"
  categories        = ["FinOps"]

  resource_results = [{
    resource_id    = "i-0abc123def456789"
    account        = "123456789012"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    location       = "us-east-1"
    resource_type  = "instance"

    result = {
      value = 5.42
    }
  }]
}

# Security risk insight with severity levels
resource "doit_insight" "open_ports" {
  key               = "open-security-groups"
  title             = "Security Groups with Open Ports"
  short_description = "Security groups allowing unrestricted inbound access"
  cloud_provider    = "aws"
  categories        = ["Security"]

  detailed_description_mdx = <<-MDX
    ## Overview
    The following security groups have inbound rules that allow unrestricted
    access (0.0.0.0/0) on sensitive ports.

    ## Remediation
    Restrict inbound rules to specific IP ranges or security groups.
  MDX

  tags = ["ISO-27001", "CIS"]

  resource_results = [{
    resource_id    = "sg-0abc123def456789"
    account        = "123456789012"
    cloud_provider = "aws"
    result_type    = "security_risk"
    severity       = "high"

    result = {
      critical = 0
      high     = 3
      medium   = 1
      low      = 0
    }
  }]
}

# Rightsizing recommendation with current and recommended state
resource "doit_insight" "rightsizing" {
  key               = "ec2-rightsizing"
  title             = "EC2 Instance Rightsizing"
  short_description = "Right-size EC2 instances based on CPU and memory utilization"
  cloud_provider    = "aws"
  categories        = ["FinOps"]

  easy_win_description = "These instances can be resized with minimal risk during the next maintenance window."

  resource_results = [{
    resource_id    = "i-0def456789abc123"
    account        = "123456789012"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings_with_recommendation"
    location       = "eu-west-1"
    resource_type  = "instance"

    result = {
      value          = 12.50
      current        = "m5.xlarge"
      recommendation = "m5.large"
    }
  }]
}
