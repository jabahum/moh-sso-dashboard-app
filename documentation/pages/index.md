---
slug: /
sidebar_label: Overview
---

# Integrated Health Portal Documentation

The Integrated Health Portal is the Ministry of Health Uganda's single sign-on portal.
It launches health information systems and hosts shared administration, RBAC,
announcements, notifications, documents, data quality, surveillance and reporting tools.

## Start here

| You are | Read first | Then |
| --- | --- | --- |
| A developer new to the project | [Getting started](../../docs/getting-started.md) | [Configuration reference](../../docs/configuration.md), [Make commands](../../docs/make.md) |
| Connecting an application or system to the portal | [Onboarding handbook](../../docs/platform-onboarding.md) | [System onboarding](../../docs/system-onboarding.md), [identity environments](../../docs/identity-environments.md) |
| Managing users, roles and access | [RBAC usage](../../docs/rbac-usage.md) | [RBAC governance](../../docs/rbac-governance.md), [health context administration](../../docs/admin/health-context-management.md) |
| Deploying or operating the portal | [Deployment](../../docs/deployment.md) | [Platform runbook](../../docs/operations/platform-runbook.md), [releasing](../../docs/releasing.md) |

## Find a guide by task

| Task | Where to start |
| --- | --- |
| Run the portal locally | Follow [getting started](../../docs/getting-started.md), choose a [startup profile](../../docs/platform-core.md#startup-profiles), and check the [Make command reference](../../docs/make.md). |
| Configure environment variables | Use the [configuration reference](../../docs/configuration.md). |
| Connect an application to the portal | Start with the [onboarding handbook](../../docs/platform-onboarding.md). Use [system onboarding](../../docs/system-onboarding.md) for system registration and [identity environments](../../docs/identity-environments.md) for client setup. |
| Configure sign-in, permissions, or health context | Read the [identity and authorization contract](../../docs/platform-identity.md), then use the [RBAC guide](../../docs/rbac-usage.md) or [health context administration guide](../../docs/admin/health-context-management.md). |
| Build a backend module | Review the [platform core boundaries](../../docs/platform-core.md), then follow the [module development guide](../../docs/module-development.md). |
| Build a frontend app | Start with [frontend development](../../frontend/docs/development.md) and follow [adding a new app](../../frontend/docs/adding-a-new-app.md). |
| Deploy, release, or troubleshoot | Use the [deployment guide](../../docs/deployment.md) for setup, the [release guide](../../docs/releasing.md) for versioned releases, and the [platform runbook](../../docs/operations/platform-runbook.md) for operations. |
| Configure notifications or scheduled jobs | Start with [notifications](../../docs/notifications.md) or [scheduler configuration](../../docs/scheduler-configuration.md). |
| Look up an endpoint or data schema | See the [API reference](../../docs/api-reference.md) and the [surveillance database guide](../../docs/surveillance-database.md). |

Each frontend app and shared package also has a README in its folder under
`frontend/apps/` and `frontend/packages/`, and the backend's implementation rules are in
`backend/README.md`.
