variable "user_id" {
  type        = string
  description = "Target user id for the single-user notification."
}

variable "allocation_ids" {
  type        = list(string)
  description = "Allocation ids included in the cost-anomaly filter."
}

variable "users" {
  type        = map(string)
  description = "Optional map of keys to user ids for the for_each example. Leave empty to skip."
  default     = {}
}
