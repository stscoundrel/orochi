# Orochi infrastructure

Terraform creates Orochi's function, two storage buckets, build image registry,
daily scheduler, and a Travers-style email alert. Resources use Orochi-specific
names and separate state from Travers.

## First deployment

Run these commands from the repository root. You need Terraform 1.10 or newer,
Python 3 for packaging, and an existing billing-enabled Google Cloud project.

1. Follow the [service-account setup guide](README.service-account.md) to create
   the deployment account, grant its project roles, and place its JSON key at
   `infra/service-account.json`.
2. Copy the settings template:

   ```sh
   cp infra/terraform.tfvars.example infra/terraform.tfvars
   ```

   Set `project_id` to the project ID, not its display name, and set `alert_email`.
   Other settings are optional; their defaults are shown in the template.
3. Package the Go source and initialize Terraform's Google provider:

   ```sh
   ./scripts/package.sh
   terraform -chdir=infra init
   terraform -chdir=infra validate
   ```

4. Preview the changes and save the plan:

   ```sh
   terraform -chdir=infra plan -out=orochi.tfplan
   ```

   Read the additions, changes, and deletions before continuing.
5. Apply that exact plan. This command starts creating resources without another
   confirmation prompt:

   ```sh
   terraform -chdir=infra apply orochi.tfplan
   ```

`-chdir=infra` makes Terraform work in the infrastructure folder. You do not need
to run it separately for each `.tf` file; Terraform reads them all together.

## Defaults and identities

The function uses Go 1.26 in `us-central1`, with 256 MiB memory, no idle instances,
and at most one instance. The scheduler runs daily at **19:45 Europe/Helsinki**,
including daylight saving changes. Its default cron expression, `45 19 * * *`,
means minute 45, hour 19, every day.

| Account | Purpose |
| --- | --- |
| Deployment key | Terraform creates and updates infrastructure. |
| `orochi-build` | Builds and publishes the function using the standard Cloud Build builder role. |
| `orochi-runtime` | Reads and writes only the event bucket. |
| `orochi-scheduler` | Invokes only Orochi's function. |

No public invoker is granted. Build images older than 30 days are cleaned up,
except for the two newest versions of each image.

## Deploying changes

Rebuild the archive whenever Go source changes, then create and review a new
plan. Terraform deploys the archive, not the source files in your working tree.

```sh
./scripts/package.sh
terraform -chdir=infra validate
terraform -chdir=infra plan -out=orochi.tfplan
terraform -chdir=infra apply orochi.tfplan
```

Run `terraform -chdir=infra init` again if provider requirements change.
After upgrading a provider, update and commit the lockfile for both local macOS
and GitHub's Linux runner:

```sh
terraform -chdir=infra providers lock -platform=darwin_amd64 -platform=linux_amd64
```

CI uses the lockfile without modifying it, so both platform checksums must be
recorded in `infra/.terraform.lock.hcl`.
The source archive contains only production Go source, `go.mod`, and `go.sum`.
Its content hash ensures changed source triggers a function rebuild.

## Verify a deployment

Use **Run now** for `orochi-daily-event-fetch` in Cloud Scheduler, then inspect
`orochi-event-fetcher` in Cloud Logging. The first successful run establishes a
silent baseline. An unchanged second run should find no new events.

Later discoveries produce event details and `Orochi found new events` in logs.
That marker increments a metric and triggers a Cloud Monitoring alert. The email
reports the alert firing rather than delivering an event digest. Check the email
channel in Monitoring if verification is requested.

For a complete notification test, use a separate test project: establish a
baseline, remove one matching event from its `events.json`, and run the scheduler
again. Expect a discovery marker and an alert; the following run should be silent.

## Local state and removal

`infra/terraform.tfstate` records which resources Terraform manages. Keep this
file backed up and out of Git; do not delete it to reset the event baseline.
The baseline itself lives in `events.json` in the event bucket.

The credential file, settings, saved plans, and state files are ignored by Git.
Terraform refuses to delete a nonempty bucket, preserving event data. Enabled
APIs remain enabled on removal because the project may be shared.
