# Deployment service account

Orochi uses a service-account JSON key to authenticate Terraform with Google
Cloud. Create this deployment account manually before deploying infrastructure.
Terraform creates the separate build, runtime, and scheduler accounts itself.

## Create the account

1. Select the Google Cloud project that will host Orochi. Billing must be enabled.
   Record its project ID; this is the value used in `terraform.tfvars`.
2. Using an account allowed to manage project access and service accounts,
   create a service account in that project. Suggested name: **Orochi Terraform**;
   suggested account ID: **orochi-terraform**.
3. Record its email address:

   ```text
   orochi-terraform@YOUR_PROJECT_ID.iam.gserviceaccount.com
   ```

4. Grant the project roles below to that email address. Apply them to the
   **Orochi project**, rather than to an individual service account or repository.

Google's console navigation may change. These steps refer to creating an IAM
service account and granting it access in the project's IAM policy, regardless
of where those controls appear in the interface.

## Required project roles

These roles cover the resources managed by Orochi's current Terraform setup.
They are a practical set of predefined administration roles, rather than a
minimal custom permission set.

| Role | Role ID | Why Terraform needs it |
| --- | --- | --- |
| Service Usage Admin | `roles/serviceusage.serviceUsageAdmin` | Enable and inspect required APIs. |
| Service Account Admin | `roles/iam.serviceAccountAdmin` | Create and manage Orochi's workload accounts. |
| Service Account User | `roles/iam.serviceAccountUser` | Attach those accounts to the function, builds, and scheduler. |
| Project IAM Admin | `roles/resourcemanager.projectIamAdmin` | Grant the project-level build role. |
| Cloud Functions Admin | `roles/cloudfunctions.admin` | Create and update the function. |
| Cloud Run Admin | `roles/run.admin` | Grant the scheduler permission to invoke the function's Cloud Run service. |
| Cloud Scheduler Admin | `roles/cloudscheduler.admin` | Create and update the scheduled job. |
| Storage Admin | `roles/storage.admin` | Manage buckets, source uploads, and bucket permissions. |
| Artifact Registry Administrator | `roles/artifactregistry.admin` | Create the image repository and configure cleanup policies. |
| Logs Configuration Writer | `roles/logging.configWriter` | Create the discovery log-based metric. |
| Monitoring Editor | `roles/monitoring.editor` | Create the alert policy and email notification channel. |

The deployment account needs these roles. Giving them only to `orochi-build`,
`orochi-runtime`, or `orochi-scheduler` does not authorize Terraform to create
resources. Terraform assigns those workload accounts their own permissions.

Service Usage API, Cloud Resource Manager API, and Identity and Access Management
(IAM) API must be available in the project. Enable them with your administrative
account if needed; Terraform enables Orochi's application APIs during deployment.

## Create and place the JSON key

1. Create a new **JSON service-account key** for the deployment account and
   download it. The account performing this step needs permission to create keys.
2. Rename the downloaded file to exactly **`service-account.json`**.
3. Place it directly inside Orochi's `infra` folder:

   ```text
   orochi/
   └── infra/
       ├── service-account.json
       ├── terraform.tfvars
       └── main.tf
   ```

The expected path from the repository root is **`infra/service-account.json`**.
The filename is ignored by Git, and the packaging script excludes JSON keys
from the function archive. Do not commit or share the key's contents.

If key creation is blocked by an organization policy, ask the organization's
administrator to allow key creation for this project or account.

## Set local variables

From the repository root:

```sh
cp infra/terraform.tfvars.example infra/terraform.tfvars
```

Edit `infra/terraform.tfvars`:

```hcl
project_id  = "YOUR_PROJECT_ID"
alert_email = "you@example.com"
```

The default `credentials_file` is `service-account.json`, resolved relative to
`infra/` when using `terraform -chdir=infra`. Only set it explicitly when using
a different key path; absolute paths are also supported:

```hcl
credentials_file = "/absolute/path/to/deployment-key.json"
```

Terraform reads this key directly. No `gcloud` login or
`GOOGLE_APPLICATION_CREDENTIALS` environment variable is required for this setup.
Continue with the [deployment instructions](README.md#first-deployment).

## Fixing permission errors

A `403` identifies the permission Terraform's deployment account is missing.
Check that the matching role is granted to that account in the correct project.
For example, `artifactregistry.repositories.create` requires **Artifact Registry
Administrator** on the project, even if the build account already has its role.

Allow a few minutes for role changes to propagate. Existing keys use the updated
permissions; you do not need to generate a new key. If an apply partly succeeded,
keep the state file and generate a fresh plan before applying again.

## Google reference documentation

- [Service-account keys](https://docs.cloud.google.com/iam/docs/keys-create-delete)
- [Service Account Admin and Service Account User roles](https://docs.cloud.google.com/iam/docs/roles-permissions/iam)
- [Function deployment requirements](https://docs.cloud.google.com/functions/docs/tutorials/terraform)
- [Artifact Registry repository creation permissions](https://docs.cloud.google.com/artifact-registry/docs/repositories/create-repos#required_roles)
