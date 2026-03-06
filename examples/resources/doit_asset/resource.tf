# Import an existing G Suite asset to manage its license count
resource "doit_asset" "gsuite_licenses" {
  id       = "g-suite-534605520"
  quantity = 10
}
