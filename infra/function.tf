# The deployed function uses this identity to access the event bucket.
resource "google_service_account" "runtime" {
  account_id   = "orochi-runtime"
  display_name = "Orochi event monitor"

  depends_on = [google_project_service.required]
}

# Cloud Build uses a separate identity to build and publish the function image.
resource "google_service_account" "build" {
  account_id   = "orochi-build"
  display_name = "Orochi function builder"

  depends_on = [google_project_service.required]
}

# Use a dedicated build identity instead of relying on project defaults.
resource "google_project_iam_member" "build" {
  project = var.project_id
  role    = "roles/cloudbuild.builds.builder"
  member  = "serviceAccount:${google_service_account.build.email}"
}

# Keep the two newest images even when they are older than the cleanup limit.
resource "google_artifact_registry_repository" "function" {
  location      = var.region
  repository_id = "orochi"
  description   = "Orochi function build images"
  format        = "DOCKER"

  cleanup_policy_dry_run = false

  cleanup_policies {
    id     = "delete-old-images"
    action = "DELETE"
    condition {
      tag_state  = "ANY"
      older_than = "2592000s" # 30 days
    }
  }

  cleanup_policies {
    id     = "keep-recent-images"
    action = "KEEP"
    most_recent_versions {
      keep_count = 2
    }
  }

  depends_on = [google_project_service.required]
}

resource "google_cloudfunctions2_function" "event_fetcher" {
  name        = "orochi-event-fetcher"
  location    = var.region
  description = "Finds and stores new events"

  build_config {
    runtime           = "go126"
    entry_point       = "FetchAndStoreEvents"
    service_account   = google_service_account.build.name
    docker_repository = google_artifact_registry_repository.function.id

    source {
      storage_source {
        bucket     = google_storage_bucket.source.name
        object     = google_storage_bucket_object.function_source.name
        generation = google_storage_bucket_object.function_source.generation
      }
    }
  }

  service_config {
    # Scale to zero between runs and process one request at a time.
    min_instance_count               = 0
    max_instance_count               = 1
    max_instance_request_concurrency = 1
    available_memory                 = "256Mi"
    timeout_seconds                  = 60
    service_account_email            = google_service_account.runtime.email
    all_traffic_on_latest_revision   = true

    environment_variables = {
      EVENT_STORAGE_BUCKET = google_storage_bucket.events.name
    }
  }

  depends_on = [
    google_project_iam_member.build,
    google_storage_bucket_iam_member.runtime_events,
  ]
}
