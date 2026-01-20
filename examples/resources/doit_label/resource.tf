# Create a simple label
resource "doit_label" "infrastructure" {
  name  = "Infrastructure"
  color = "blue"
}

# Create a label for cost tracking
resource "doit_label" "cost_center_engineering" {
  name  = "Cost Center: Engineering"
  color = "teal"
}

# Create a label for environment categorization
resource "doit_label" "production" {
  name  = "Production"
  color = "rosePink"
}
