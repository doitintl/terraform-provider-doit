# Import using the composite ID: service_account_id/token_id.
# The access_token is not retrievable after import and stays null.
terraform import doit_service_account_token.example [service_account_id]/[token_id]
