# GitHub Runner SDK for Workshop

A just-in-time runner for GitHub Actions that runs workflow jobs inside a local
workshop. This SDK simplifies local testing of GitHub workflows, enables
interactive debugging of failed jobs, and allows testing with custom hardware
without managing persistent runner infrastructure.

---

## Reference workshop

A minimal workshop:

```yaml
# workshop.yaml
name: ci
base: ubuntu@24.04
sdks:
  - name: github-runner
    channel: 24.04/edge
```

This provides a basic runner environment. Add additional SDKs as needed for
your workflows (e.g., `docker` for container operations, `go` for Go projects).

---

## Using the SDK

### Prerequisites, project layout

1. No prerequisite SDKs are required, but you may want to add others based on
   your workflow needs (e.g., `docker`, `go`, `node`).
2. Admin-level permissions are required on the target repository to add a
   runner. Users without admin rights can fork the repository and test
   workflows in the fork. Adding a runner to an organization doesn't require
   admin rights on the organization itself, but grants access to organization
   secrets—proceed with caution.
3. Your project with GitHub Actions workflows should be in your project
   directory:

   ```bash
   git clone <YOUR_REPO_URL>
   ```

### Authorization

On first run, the `github-runner` script requests authorization using a
one-time code. This authorization is mediated by a GitHub App provided by the
Workshop team to limit SDK access to only the necessary repositories.

To grant access:

1. Navigate to the [GitHub App for
   github-runner](https://github.com/apps/test-app-jonathan-conder-1) and
   install it.
2. Configure which repositories or organizations the App can access. This can
   be changed at any time.
3. If the workshop or host machine is compromised, uninstall the App to revoke
   access immediately.

For individuals, install the App on your personal account and grant access to
the required repositories.

For organizations, install the App on the organization. After adding a runner
to the organization, workflows can use it even if the App is denied access to
the specific repository. Alternatively, add the runner to individual
repositories within the organization and grant the App access to those
repositories.

The SDK doesn't share information with Canonical or any third party (apart from
GitHub). If you prefer a different authentication mechanism, export the
`GITHUB_TOKEN` environment variable inside the workshop—the `github-runner`
script will use it if available.

### Start the runner

Once the workshop is ready and authorization is configured:

```bash
workshop exec ci github-runner --label=workshop <OWNER>[/<REPO>]
```

Replace `<OWNER>/<REPO>` with the full repository name (e.g.,
`canonical/workshop`). If omitted, the script tries to detect this from the
local repository. For organization-level runners, provide only the organization
name (e.g., `canonical`).

The `--label` option adds a label to distinguish this runner from GitHub-hosted
runners and other self-hosted runners. Use `--help` to see all available
options.

After a few seconds, the runner will be ready to accept jobs.

### Configure workflows to use the runner

Add the runner label to the `runs-on` option in your workflow. We recommend
making this configurable to avoid repeatedly editing the workflow:

```yaml
# .github/workflows/test.yaml
on:
  pull_request:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      runner:
        description: Where to run the job
        type: choice
        required: true
        options: [ubuntu-latest, workshop]
        default: ubuntu-latest

jobs:
  test:
    runs-on: ["${{ inputs.runner || 'ubuntu-latest' }}"]
    steps:
      - uses: actions/checkout@v4
      - run: make test
```

Commit the updated workflow, find it in the Actions tab of the repository, and
select "Run workflow" with the runner set to `workshop`.

The Runner client prints logs when jobs start and finish. Full logs remain
viewable on GitHub.

### Running multiple jobs in parallel

The Runner client runs one job at a time. To run several jobs in parallel, use
multiple workshops:

```bash
mkdir -p .workshop
mv workshop.yaml .workshop/ci.yaml
sed 's/name: ci/name: ci2/' <.workshop/ci.yaml >.workshop/ci2.yaml
workshop launch ci2
workshop exec ci2 github-runner --label=workshop
```

### Ensuring clean state between jobs

The Runner client doesn't clean up after itself, which aids debugging but may
cause issues for some workflows. To ensure a clean state, refresh the workshop
after each job:

```bash
while workshop exec ci github-runner --label=workshop --once; do
  workshop refresh ci
done
```

### Branch-conditional runners

For quick iteration, make the runner conditional on the branch name:

```yaml
# .github/workflows/test.yaml
on:
  push:
    branches:
      - main
      - workshop-runner/**

jobs:
  test:
    runs-on: ["${{ startsWith(github.ref_name, 'workshop-runner/') && 'workshop' || 'ubuntu-latest' }}"]
    steps:
      - uses: actions/checkout@v4
      - run: make test
```

Push to a `workshop-runner/*` branch to automatically use your local runner.

### Important notes

- Take care with logging: some actions may leak sensitive information about the
  runner (e.g., IP address).
- In rare cases (like a power outage during registration), runners can remain
  attached to the repository indefinitely. Remove these manually in the
  repository or organization settings.
- To inspect logs and files after a failed run, enter the workshop shell after
  the job completes and navigate to `~/actions-runner/_work`.

---

## Plugs (resources this SDK consumes)

### `tool-cache`

- Interface: `mount`
- Workshop target: `/home/workshop/actions-runner/_work/_tool`
- Purpose: Persists tools downloaded by setup actions (e.g., `setup-python`,
  `setup-node`) between workshop updates, avoiding repeated downloads and
  improving job startup time.

## Slots (resources this SDK provides)

This SDK doesn't define any slots.

---

## Documentation and guidance

- [GitHub Actions documentation](https://docs.github.com/en/actions)
- [Self-hosted runners](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners/about-self-hosted-runners)
- [Just-in-time runners](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions#using-just-in-time-runners)
- [Official Runner repository](https://github.com/actions/runner)
- [Workshop documentation](https://canonical-workshop.readthedocs-hosted.com/latest/)

---

## Community and support

- GitHub Actions community: [GitHub Community Discussions](https://github.com/orgs/community/discussions/categories/actions)
- Workshop forum: [Workshop Discourse](https://discourse.canonical.com/c/engineering/workshops/34)
- Please review our [Code of Conduct](https://ubuntu.com/community/ethos/code-of-conduct) before participating.

---

## Contributions

All contributions, including code, documentation updates, and issue reports,
are welcome!

- See `CONTRIBUTING.md` for guidelines.
- Open issues or pull requests on the official repository.

---

## License and copyright

Copyright 2025 Canonical Ltd.

This SDK is licensed under the [MIT License](https://opensource.org/licenses/MIT),
the same license as [GitHub Actions Runner](https://github.com/actions/runner/blob/main/LICENSE).
