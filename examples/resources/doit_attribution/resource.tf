# Manage Attribution group
resource "doit-console_attribution" "attri" {
  name        = "attritestnewname9"
  description = "attritestdiana8"
  formula     = "A"
  components  = [{ type = "label", key = "iris_location", values = ["us"] }]
}