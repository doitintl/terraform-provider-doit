# Local `doit_user_notification` exercise

Runnable Terraform against a local or remote DoiT API. Separate from the registry docs example under `examples/resources/doit_user_notification`.

## Prerequisites

1. Build and install the provider from this worktree:

   ```bash
   go install .
   ```

2. Copy local secrets (do not commit them):

   ```bash
   cp terraform.tfvars.example terraform.tfvars
   cp dev.tfrc.example dev.tfrc
   ```

   Fill `user_id` and `allocation_ids` in `terraform.tfvars`. Point `dev.tfrc` at your `go install` bin if needed (default: `/Users/piotrbalwierczak/go/bin`).

3. Run the IAM API handlers locally (until deployed to api-dev):

   ```bash
   cd omni/worktrees/feature/CMP-54312-api/services/console-iam
   make run   # PORT=8200
   ```

## Apply

```bash
export TF_CLI_CONFIG_FILE="$PWD/dev.tfrc"
export DOIT_HOST="http://localhost:8200"
export DOIT_API_TOKEN="<usersManager token>"
# export DOIT_CUSTOMER_CONTEXT="<tenant-id>"  # only when the token can reach more than one customer

terraform init
terraform plan
terraform apply
terraform plan -detailed-exitcode
```

Then change `allocation_ids`, apply again, destroy, and import:

```bash
./import.sh '<user-id>' '<notification-id>'
```

For two users, set `users` in `terraform.tfvars` and apply twice; the second apply must not create duplicates.
