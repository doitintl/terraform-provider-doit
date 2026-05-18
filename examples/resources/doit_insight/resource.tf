# Manage custom insight metadata through the DoiT Insights API.
# Insights define optimization findings (cost savings, security risks, etc.)
# that are displayed in the DoiT console.
#
# Resource results (the actual findings) are managed separately using the
# doit_insight_resource_results resource, which references the insight by key.

# Basic cost savings insight
resource "doit_insight" "unused_instances" {
  key               = "unused-ec2-instances"
  title             = "Unused EC2 Instances"
  short_description = "EC2 instances with consistently low CPU utilization"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

# Security insight with detailed description
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
}

# Rightsizing insight with all optional metadata fields
resource "doit_insight" "rightsizing" {
  key               = "ec2-rightsizing"
  title             = "EC2 Instance Rightsizing"
  short_description = "Right-size EC2 instances based on CPU and memory utilization"
  cloud_provider    = "aws"
  categories        = ["FinOps"]

  easy_win_description = "These instances can be resized with minimal risk during the next maintenance window."
}
