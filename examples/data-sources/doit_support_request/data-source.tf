# Retrieve a single support request by ID
data "doit_support_request" "example" {
  ticket_id = 12345
}

output "subject" {
  value = data.doit_support_request.example.subject
}

output "status" {
  value = data.doit_support_request.example.status
}
