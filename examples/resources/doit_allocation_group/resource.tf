# Create allocations for Japan and Germany in a K8s project
resource "doit_allocation" "name" {
  name = "Japan in K8s project"
  rule = {
    formula = "A AND B"
    components = [{
      key    = "country"
      mode   = "is"
      type   = "fixed"
      values = ["JP"]
      },
      {
        key    = "project_id"
        mode   = "is"
        type   = "fixed"
        values = ["test-k8s-project-468707"]
    }]
  }
}

resource "doit_allocation" "name2" {
  name = "Germany in K8s project"
  rule = {
    formula = "A AND B"
    components = [{
      key    = "country"
      mode   = "is"
      type   = "fixed"
      values = ["DE"]
      },
      {
        key    = "project_id"
        mode   = "is"
        type   = "fixed"
        values = ["test-k8s-project-468707"]
    }]
  }
}

# Create an allocation group for the allocations, referencing the allocations for Germany and Japan and adding another one in-line for the US
resource "doit_allocation_group" "this" {
  name = "My Allocation Group"
  rules = [
    {
      action = "select"
      id     = doit_allocation.name.id
      components = doit_allocation.name.rule.components
      formula    = doit_allocation.name.rule.formula
    },
    {
      action = "select"
      id     = doit_allocation.name2.id
      components = doit_allocation.name2.rule.components
      formula    = doit_allocation.name2.rule.formula
    },
    {
      action      = "create"
      name        = "test-rule-%d"
      description = "Terraform test rule"
      components = [
        {
          key    = "country"
          mode   = "is"
          type   = "fixed"
          values = ["US"]
        },
        {
          key    = "project_id"
          mode   = "is"
          type   = "fixed"
          values = ["test-k8s-project-468707"]
        },
      ],
      formula = "A AND B"
    },
  ]
}
