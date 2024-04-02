# Manage Attribution group
resource "doit_attribution" "attri" {
  name        = "attritestname"
  description = "attritestdesc"
  formula     = "A"
  components  = [{ type = "label", key = "iris_location", values = ["us"] }]
}