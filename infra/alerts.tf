# Same notification chain as Travers: log marker -> metric -> email alert.
resource "google_logging_metric" "new_events" {
  name        = "orochi_new_events"
  description = "Counts Orochi runs that discover new events"
  filter      = <<-FILTER
    resource.type="cloud_run_revision"
    resource.labels.project_id="${var.project_id}"
    resource.labels.location="${var.region}"
    resource.labels.service_name="${google_cloudfunctions2_function.event_fetcher.name}"
    textPayload =~ "Orochi found new events"
  FILTER

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    unit        = "1"
  }

  depends_on = [google_project_service.required]
}

resource "google_monitoring_notification_channel" "email" {
  display_name = "Orochi notification"
  type         = "email"

  labels = {
    email_address = var.alert_email
  }

  depends_on = [google_project_service.required]
}

resource "google_monitoring_alert_policy" "new_events" {
  display_name = "Orochi found new events"
  combiner     = "OR"

  documentation {
    subject = "Orochi found new events"
    content = "Orochi discovered new Helsinki shooting events. See the orochi-event-fetcher logs for event details and registration links."
  }

  conditions {
    display_name = "Discovery marker logged"

    condition_threshold {
      filter          = <<-FILTER
        metric.type="logging.googleapis.com/user/${google_logging_metric.new_events.name}"
        AND resource.type="cloud_run_revision"
      FILTER
      comparison      = "COMPARISON_GT"
      threshold_value = 0
      duration        = "0s"

      # Count discovery runs in a one-minute window; any count above zero alerts.
      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_SUM"
      }

      trigger {
        count = 1
      }
    }
  }

  # Close an incident after 30 minutes without data so future runs can alert again.
  alert_strategy {
    auto_close = "1800s"
  }

  notification_channels = [google_monitoring_notification_channel.email.name]
}
