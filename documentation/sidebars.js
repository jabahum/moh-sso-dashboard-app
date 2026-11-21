export default {
  // Group guides by reader task while retaining their existing document IDs and URLs.
  guides: [
    'documentation/pages/index',
    {
      type: 'category',
      label: 'Getting started',
      items: [
        'docs/platform-development',
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
        'docs/rbac-usage',
        'docs/rbac-governance',
        'docs/rbac-groups',
        'docs/rbac-keycloak-sync',
        'docs/standard-user-defaults',
        'docs/architecture/health-context',
        'docs/admin/health-context-management',
        'docs/integrations/health-context-mapping',
        'docs/operations/health-context-sync',
        'docs/security/contextual-authorization',
        'docs/architecture/health-context-audit',
      ],
    },
    {
      type: 'category',
      label: 'Platform architecture & modules',
      items: [
        'docs/platform-core',
        'docs/modular-distribution',
        'docs/module-coverage',
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
        'docs/backend-microservice-readiness',
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
            'frontend/docs/microfrontend-strategy',
            'frontend/docs/import-maps',
            'frontend/docs/import-map-strategy',
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
          label: 'Build, release & verification',
          items: [
            'frontend/docs/build-and-deployment',
            'frontend/docs/package-publishing',
            'frontend/docs/versioning',
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
        'docs/ci-checks',
        'docs/versioning',
        'docs/releasing',
        'docs/releasing-backend',
        'docs/rollback',
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
