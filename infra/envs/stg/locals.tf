locals {
  environment = "stg"

  common_tags = {
    Project     = var.project_name
    Environment = local.environment
    ManagedBy   = "Terraform"
    Repository  = "devlab-board"
  }
}
