# Helm Management

Kite provides basic Helm management in the dashboard, covering chart discovery, release installation, upgrade, rollback, and uninstall.

## App Catalog

Open **App Catalog** from the sidebar to browse Helm charts.

Kite supports two chart sources:

- **Artifact Hub**: search public Helm charts.
- **Repositories**: browse Helm repositories managed in Kite.

::: tip
When using the Artifact Hub source, Kite may request Artifact Hub to fetch chart lists and chart details.
:::

::: warning
Kite only displays chart information and is not responsible for the chart content. Review chart details, templates, and values carefully before installing or upgrading.
:::

Users with the **admin** role can add or remove Helm repositories. Removing a repository only removes it from Kite and does not uninstall existing releases. If any scheduled auto-upgrade tasks reference the repository, Kite automatically disables those tasks to prevent them from attempting upgrades against a non-existent repository. The tasks are disabled rather than deleted, so their history and configuration are preserved.

Open a chart to view its README, values, templates, and versions. If the chart package is available, you can install it directly from Kite.

### Search and Pagination

Both chart sources use server-side pagination and search. The search box sends the query to the backend, which filters charts by name, description, keywords, and other metadata before returning results. Page size options are 10, 20, and 50.

Search input is debounced (400ms) before triggering an API request. This is especially important for the Artifact Hub source, where each search queries an external API — debouncing prevents excessive requests while typing.

### Cache and Refresh

Kite caches the repository index (`index.yaml`) for 5 minutes and chart content (README, values, templates) for 10 minutes to speed up browsing.

When a repository publishes new chart versions, they may not appear immediately because of the cache. To force Kite to fetch the latest data:

- Click the **Refresh** button on the App Catalog page. This sends a `refresh=true` query parameter that bypasses the cache and re-downloads the index and chart content.
- Opening the upgrade dialog also triggers a refresh automatically, ensuring you see the latest available versions when upgrading.

### Fault Tolerance

When listing charts from multiple repositories, if a single repository is unreachable (network error, invalid index, etc.), Kite logs a warning and skips that repository instead of failing the entire chart list. Charts from other healthy repositories are still displayed normally.

## Helm Releases

Open **Helm Release** from the sidebar to view installed releases.

The release detail page shows release status, chart version, values, resources, history, logs, and rendered manifests.

Kite supports dry-run previews before install and upgrade. You can upgrade a release from the detail page, roll back from the history tab, or delete a release to uninstall it from the cluster.

### Values Merge Preview

Both install and upgrade dialogs display three columns:

- **Default Values**: The chart's built-in `values.yaml` (read-only).
- **Custom Values**: Your overrides, editable as YAML.
- **Merged Values Preview**: The result of merging chart defaults with your custom values and `--set` overrides, computed using Helm's coalesce logic. This shows exactly what values will be applied to the release.

The merged preview updates automatically (with a short debounce) as you edit custom values or `--set` parameters, so you can verify overrides before running a dry run or install.

### Version Selection in Install Dialog

The install dialog includes a version selector that lists all available chart versions. By default, the latest version is selected. You can choose a different version from the dropdown — Kite will fetch the chart details and default values for the selected version, and the merged values preview updates accordingly.

The version dropdown shows the version number, AppVersion, and publication date for each entry. The current (default) version is marked with a "Current" label.

### Namespace Selection in Install Dialog

The install dialog uses a unified namespace selector that combines browsing existing namespaces with creating new ones. You can search for an existing namespace by name, or type a new name and select the "Create" option that appears. When a non-existent namespace is selected, a "Create namespace" checkbox appears inline, allowing Helm to create the namespace during installation. Selecting an existing namespace hides the checkbox since creation is unnecessary.

### Set Values and Advanced Options

Both install and upgrade dialogs support Helm `--set` values for quick overrides without editing YAML. Click **Advanced settings** in the dialog to expand the section:

- **Set values (--set)**: Enter one `key=value` per line, using Helm `--set` syntax (e.g. `image.tag=v2.0.0`, `replicas=3`, `servers[0].port=8080`). These are merged on top of the YAML values.
- **force-conflicts**: Force-apply changes by overwriting conflicting fields (server-side apply).
- **wait**: Wait for all Kubernetes resources to be ready before completing.
- **Rollback on failure**: Automatically roll back the release if the operation fails.

::: tip
Set values take precedence over YAML values. Use them for quick overrides without modifying the full values file.
:::

### Asynchronous Operations

Install, upgrade, rollback, and uninstall are executed asynchronously to avoid HTTP timeout on long-running operations:

1. The API returns `202 Accepted` with a task ID and initial status (`pending`).
2. The operation runs in a background goroutine. The task transitions through `pending` → `running` → `succeeded` or `failed`.
3. Poll the task status via `GET /api/v1/helmrelease/tasks/:taskID` until the status reaches `succeeded` or `failed`.
4. The frontend automatically polls every 2 seconds and updates the UI when the task completes.

Dry-run operations remain synchronous because they are fast and return preview results immediately.

::: tip
If a task fails, the error message is stored in the task record and displayed in the UI. The task record is persisted in the database for auditing.
:::

### Auto-Upgrade Error Handling

When configuring an auto-upgrade schedule for a release, if the target release does not exist in the cluster, the API returns `404 Not Found` instead of `500 Internal Server Error`. This makes it clear that the release does not exist rather than indicating an internal server problem.

## Permissions

Repository management requires the **admin** role. Release operations are controlled by Kite RBAC through the `helmrelease` resource (`get`, `create`, `update`, `delete`).

::: warning
Grant `helmrelease` permissions carefully. Helm actions are executed by Kite with the cluster credentials configured in Kite, so users with `helmrelease` `create`, `update`, or `delete` permissions may create, update, or delete chart-rendered resources even if their own Kubernetes RBAC permissions would not allow those direct operations.

The cluster credentials configured in Kite also need enough Kubernetes permissions for the resources rendered by the chart.
:::
