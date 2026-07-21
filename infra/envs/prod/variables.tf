variable "aws_region" {
  description = "AWS region for the environment."
  type        = string
}

variable "project_name" {
  description = "Project name used for naming and tags."
  type        = string
  default     = "devlab-board"
}
