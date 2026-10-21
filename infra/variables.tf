variable "project_id" {
  description = "ID of an existing, billing-enabled Google Cloud project (not its display name)."
  type        = string
}

variable "alert_email" {
  description = "Email address for Cloud Monitoring alert notifications."
  type        = string
  sensitive   = true
}

variable "region" {
  description = "Google Cloud region for the function, scheduler, storage, and build images."
  type        = string
  default     = "us-central1"
}

variable "schedule" {
  description = "Cron schedule in Europe/Helsinki time. The default runs daily at 19:45."
  type        = string
  default     = "45 19 * * *"
}

variable "credentials_file" {
  description = "Deployment key path relative to infra/, or an absolute path."
  type        = string
  default     = "service-account.json"
}
