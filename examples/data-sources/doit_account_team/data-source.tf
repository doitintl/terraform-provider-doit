# List your DoIT account team
data "doit_account_team" "team" {}

# Output account manager names and emails
output "account_managers" {
  value = [for am in data.doit_account_team.team.account_managers : {
    name  = am.name
    email = am.email
    role  = am.role
  }]
}
