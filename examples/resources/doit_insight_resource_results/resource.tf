# Manage resource-level findings (resource results) for a DoiT Insight.
# Each doit_insight_resource_results block is tied to a specific insight
# via source_id + insight_key and contains one or more resource results.
#
# The insight itself must be created first using the doit_insight resource.

# Basic cost savings — potential daily savings per resource
resource "doit_insight" "unused_instances" {
  key               = "unused-ec2-instances"
  title             = "Unused EC2 Instances"
  short_description = "EC2 instances with consistently low CPU utilization"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "unused_instances" {
  source_id   = "public-api"
  insight_key = doit_insight.unused_instances.key

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

# Security risk — severity counts per resource
resource "doit_insight" "open_ports" {
  key               = "open-security-groups"
  title             = "Security Groups with Open Ports"
  short_description = "Security groups allowing unrestricted inbound access"
  cloud_provider    = "aws"
  categories        = ["Security"]
}

resource "doit_insight_resource_results" "open_ports" {
  source_id   = "public-api"
  insight_key = doit_insight.open_ports.key

  resource_results = [{
    resource_id    = "sg-0abc123def456789"
    account        = "123456789012"
    cloud_provider = "aws"
    result_type    = "security_risk"
    resource_type  = "security-group"

    result = {
      critical = 0
      high     = 3
      medium   = 1
      low      = 0
    }
  }]
}

# Rightsizing recommendation — savings with current and recommended state
resource "doit_insight" "rightsizing" {
  key               = "ec2-rightsizing"
  title             = "EC2 Instance Rightsizing"
  short_description = "Right-size EC2 instances based on CPU and memory utilization"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "rightsizing" {
  source_id   = "public-api"
  insight_key = doit_insight.rightsizing.key

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

# Multiple results in a single resource — all results for one insight
resource "doit_insight" "idle_ebs" {
  key               = "idle-ebs-volumes"
  title             = "Idle EBS Volumes"
  short_description = "Unattached or idle EBS volumes incurring costs"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "idle_ebs" {
  source_id   = "public-api"
  insight_key = doit_insight.idle_ebs.key

  resource_results = [
    {
      resource_id    = "vol-0abc123def456789"
      account        = "123456789012"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      location       = "us-east-1"
      resource_type  = "volume"

      result = {
        value = 3.20
      }
    },
    {
      resource_id    = "vol-0def789abc123456"
      account        = "123456789012"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      location       = "us-west-2"
      resource_type  = "volume"

      result = {
        value = 1.80
      }
    },
  ]
}

# All optional fields — external ID, external URL, and metadata
resource "doit_insight" "detailed_findings" {
  key               = "detailed-findings"
  title             = "Detailed Findings"
  short_description = "Example with all optional resource result fields"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "detailed_findings" {
  source_id   = "public-api"
  insight_key = doit_insight.detailed_findings.key

  resource_results = [{
    resource_id    = "i-all-fields-001"
    account        = "123456789012"
    cloud_provider = "aws"
    result_type    = "potential_daily_savings"
    location       = "us-east-1"
    resource_type  = "instance"
    external_id    = "ext-abc-123"
    external_url   = "https://console.aws.amazon.com/ec2/i-all-fields-001"

    result = {
      value = 5.42
    }
  }]
}
