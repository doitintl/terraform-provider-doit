#!/usr/bin/env bash
set -euo pipefail

# Usage: ./import.sh <user-id> <notification-id>
terraform import 'doit_user_notification.test' "${1:?user-id}/${2:?notification-id}"
