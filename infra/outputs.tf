output "function_url" {
  description = "Authenticated HTTP endpoint for the event monitor."
  value       = google_cloudfunctions2_function.event_fetcher.service_config[0].uri
}

output "event_bucket" {
  description = "Bucket containing events.json and its baseline."
  value       = google_storage_bucket.events.name
}

output "scheduler_job" {
  description = "Daily scheduler job name."
  value       = google_cloud_scheduler_job.daily_event_fetch.name
}
