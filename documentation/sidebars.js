export default {
  // Group guides by reader task while retaining their existing document IDs and URLs.
  // Each category starts with the page a new reader should open first.
  // frontend/docs/import-map-strategy and microfrontend-strategy are kept only so
  // older links resolve; they are deliberately left out of the sidebar.
  guides: [
    'documentation/pages/index',
    {
      type: 'category',
      label: 'Getting started',
      items: [
        'docs/getting-started',
        'docs/configuration',
        'docs/local-healthbi-startup',
        'docs/make',
        'docs/tooling',
      ],
    },
    {
      type: 'category',
      label: 'Application onboarding',
      items: [
        {
          type: 'doc',
          id: 'docs/platform-onboarding',
          label: 'Onboarding handbook',
        },
        'docs/system-onboarding',
        'docs/application-manifests',
        'docs/identity-environments',
        'docs/cross-application-sso',
        'docs/external-app-header',
        'docs/keycloak-digital-timesheets',
        'docs/templates/application-onboarding-record',
        'docs/images/platform-onboarding/README',
      ],
    },
    {
      type: 'category',
      label: 'Identity, access & health context',
      items: [
        {
          type: 'doc',
          id: 'docs/platform-identity',
          label: 'Identity and authorization contract',
        },
        {
          type: 'category',
          label: 'Roles and permissions (RBAC)',
          items: [
            'docs/rbac-usage',
            'docs/rbac-governance',
            'docs/rbac-groups',
            'docs/rbac-keycloak-sync',
            'docs/standard-user-defaults',
          ],
        },
        {
          type: 'category',
          label: 'Identity sync (CLIMS, POES)',
          items: [
            {
              type: 'doc',
              id: 'docs/integrations/client-onboarding',
              label: 'Onboarding a client application',
            },
            {
              type: 'doc',
              id: 'docs/integrations/platform-api',
              label: 'Platform API for application backends',
            },
            {
              type: 'doc',
              id: 'docs/integrations/connected-applications',
              label: 'Connected applications',
            },
            {
              type: 'doc',
              id: 'docs/integrations/alerts-integration',
              label: 'Connecting Alerts Manager',
            },
            {
              type: 'doc',
              id: 'docs/integrations/local-integration-testing',
              label: 'Testing connected applications locally',
            },
            'docs/integrations/identity-sync-contract',
            'docs/integrations/identity-sync-adapter',
            'docs/architecture/identity-sync',
            'docs/architecture/application-events',
            'docs/operations/identity-sync-pilot',
          ],
        },
        {
          type: 'category',
          label: 'Health context',
          items: [
            'docs/architecture/health-context',
            'docs/admin/health-context-management',
            'docs/integrations/health-context-mapping',
            'docs/operations/health-context-sync',
            'docs/security/contextual-authorization',
            'docs/architecture/health-context-audit',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Platform architecture & modules',
      items: [
        'docs/platform-core',
        {
          type: 'doc',
          id: 'docs/module-development',
          label: 'Module development',
        },
        {
          type: 'doc',
          id: 'docs/module-operations',
          label: 'Module operations',
        },
        'docs/modular-distribution',
        'docs/module-coverage',
        'docs/backend-microservice-readiness',
        {
          type: 'doc',
          id: 'docs/platform-development',
          label: 'Platform development phases',
        },
      ],
    },
    {
      type: 'category',
      label: 'Frontend development',
      items: [
        'frontend/docs/development',
        'frontend/docs/architecture',
        'frontend/docs/adding-a-new-app',
        {
          type: 'category',
          label: 'Microfrontends',
          items: [
            'frontend/docs/microfrontends',
            'frontend/docs/import-maps',
            'frontend/docs/module-source-strategy',
            'frontend/docs/multi-framework-apps',
            'frontend/docs/non-react-microfrontends',
            'frontend/docs/microfrontend-dependency-audit',
          ],
        },
        {
          type: 'category',
          label: 'Shell & user experience',
          items: [
            'frontend/docs/shell-ux',
            'frontend/docs/pwa',
            'frontend/docs/responsive-audit',
          ],
        },
        {
          type: 'category',
          label: 'Build, publish & verify',
          items: [
            'frontend/docs/build-and-deployment',
            'frontend/docs/package-publishing',
            {
              type: 'doc',
              id: 'frontend/docs/versioning',
              label: 'Frontend versioning',
            },
            'frontend/docs/release-ci',
            'frontend/docs/verification',
            'frontend/docs/troubleshooting',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Deployment & operations',
      items: [
        'docs/deployment',
        'docs/operations/platform-runbook',
        'docs/operations/portal-hardening',
        'docs/cd-pipeline',
        'docs/rollback',
      ],
    },
    {
      type: 'category',
      label: 'Releases & CI',
      items: [
        'docs/releasing',
        {
          type: 'doc',
          id: 'docs/releasing-backend',
          label: 'Backend releases',
        },
        {
          type: 'doc',
          id: 'docs/versioning',
          label: 'Versioning and image tags',
        },
        'docs/ci-checks',
      ],
    },
    {
      type: 'category',
      label: 'Notifications & scheduling',
      items: [
        'docs/notifications',
        'docs/platform-notifications',
        'docs/operations/notification-delivery-runbook',
        'docs/sms-delivery',
        'docs/scheduler-configuration',
        'docs/scheduler-backend',
      ],
    },
    {
      type: 'category',
      label: 'API & data reference',
      items: [
        'docs/api-reference',
        'docs/surveillance-database',
      ],
    },
  ],
};
