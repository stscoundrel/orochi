# The scheduler only needs permission to invoke Orochi.
resource "google_service_account" "scheduler" {
  account_id   = "orochi-scheduler"
  display_name = "Orochi scheduled invoker"

  depends_on = [google_project_service.required]
}

# Second-generation functions use Cloud Run invoker permissions.
resource "google_cloud_run_service_iam_member" "scheduler_invoker" {
  project  = var.project_id
  location = var.region
  service  = google_cloudfunctions2_function.event_fetcher.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.scheduler.email}"
}

resource "google_cloud_scheduler_job" "daily_event_fetch" {
  name             = "orochi-daily-event-fetch"
  description      = "Checks for new Helsinki shooting events"
  region           = var.region
  schedule         = var.schedule # Default: 19:45 local time, including daylight saving changes.
  time_zone        = "Europe/Helsinki"
  attempt_deadline = "90s"

  # Retry failed requests twice; successful runs do not need another invocation.
  retry_config {
    retry_count          = 2
    min_backoff_duration = "60s"
    max_backoff_duration = "300s"
    max_doublings        = 2
  }

  http_target {
    uri         = google_cloudfunctions2_function.event_fetcher.service_config[0].uri
    http_method = "GET"

    oidc_token {
      audience              = google_cloudfunctions2_function.event_fetcher.service_config[0].uri
      service_account_email = google_service_account.scheduler.email
    }
  }

  # Set up permissions and notifications before the job can start running.
  depends_on = [
    google_cloud_run_service_iam_member.scheduler_invoker,
    google_monitoring_alert_policy.new_events,
  ]
}
