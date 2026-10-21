# Holds the packaged Go source used during deployment.
resource "google_storage_bucket" "source" {
  name                        = "${var.project_id}-orochi-source"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false # Do not delete a bucket that still contains data.

  depends_on = [google_project_service.required]
}

# A new source hash creates a new object and triggers a function rebuild.
resource "google_storage_bucket_object" "function_source" {
  name   = "orochi-${filesha256(local.source_archive)}.zip"
  bucket = google_storage_bucket.source.name
  source = local.source_archive
}

# Holds events.json, including the baseline and previously seen event IDs.
resource "google_storage_bucket" "events" {
  name                        = "${var.project_id}-orochi-events"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false # Do not delete a bucket that still contains data.

  depends_on = [google_project_service.required]
}

resource "google_storage_bucket_iam_member" "runtime_events" {
  bucket = google_storage_bucket.events.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.runtime.email}"
}
