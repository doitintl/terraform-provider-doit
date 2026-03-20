# Retrieve all comments on a support request
data "doit_support_request_comments" "example" {
  ticket_id = 12345
}

output "comment_count" {
  value = length(data.doit_support_request_comments.example.comments)
}
