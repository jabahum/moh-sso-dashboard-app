import { Component, type ReactNode } from "react";
import { InlineLoading, Tab, TabList, TabPanel, TabPanels, Tabs } from "@carbon/react";

import { PageShell } from "@moh-sso/ui";
import { PermissionGuard, PERMISSIONS, useAuthorization } from "@moh-sso/auth";

import { useGetTemplatesQuery, useListDocumentsQuery } from "../../api";
import { TemplatesTab } from "../../components/TemplatesTab";
import { DocumentsTab } from "../../components/DocumentsTab";
import "./documents.scss";

type PageErrorBoundaryState = {
  error: Error | null;
};

class PageErrorBoundary extends Component<{ children: ReactNode }, PageErrorBoundaryState> {
  state: PageErrorBoundaryState = {
    error: null,
  };

  static getDerivedStateFromError(error: Error): PageErrorBoundaryState {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <div className="documents-page__error">
          <strong>Page crashed:</strong>

          <pre className="documents-page__error-details">
            {this.state.error.message}
            {"\n\n"}
            {this.state.error.stack}
          </pre>
        </div>
      );
    }

    return this.props.children;
  }
}

function DocumentPageContent() {
  const { can } = useAuthorization();
  // The first tab the user can see decides when the page is ready.
  const showsTemplates = can(PERMISSIONS.documentTemplatesRead);
  const templatesQuery = useGetTemplatesQuery(undefined, { skip: !showsTemplates });
  const documentsQuery = useListDocumentsQuery(undefined, { skip: showsTemplates });
  const isLoading = showsTemplates ? templatesQuery.isLoading : documentsQuery.isLoading;

  // Like Report Scheduler: only a loading indicator until the first data is ready.
  if (isLoading) return <InlineLoading description="Loading documents…" />;

  return (
    <PageErrorBoundary>
      <div className="documents-page">
        <PageShell
          title="Document Management"
          subtitle="Manage reusable upload templates and monitor uploaded documents."
        >
          <div className="documents-page__tabs">
            <Tabs>
              <TabList
                aria-label="Document management tabs"
                contained
                className="documents-page__tab-list"
              >
                <PermissionGuard permission={PERMISSIONS.documentTemplatesRead}>
                  <Tab>Templates</Tab>
                </PermissionGuard>
                <PermissionGuard permission={PERMISSIONS.documentsRead}>
                  <Tab>Template Data Upload</Tab>
                </PermissionGuard>
              </TabList>

              <TabPanels>
                <PermissionGuard permission={PERMISSIONS.documentTemplatesRead}>
                  <TabPanel className="documents-page__tab-panel">
                    <PageErrorBoundary>
                      <TemplatesTab />
                    </PageErrorBoundary>
                  </TabPanel>
                </PermissionGuard>

                <PermissionGuard permission={PERMISSIONS.documentsRead}>
                  <TabPanel className="documents-page__tab-panel">
                    <PageErrorBoundary>
                      <DocumentsTab />
                    </PageErrorBoundary>
                  </TabPanel>
                </PermissionGuard>
              </TabPanels>
            </Tabs>
          </div>
        </PageShell>
      </div>
    </PageErrorBoundary>
  );
}

export default function DocumentPage() {
  return (
    <PermissionGuard anyOf={[PERMISSIONS.documentsRead, PERMISSIONS.documentTemplatesRead]}>
      <DocumentPageContent />
    </PermissionGuard>
  );
}
