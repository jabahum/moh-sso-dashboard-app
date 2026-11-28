import type { MicrofrontendRuntimeProps } from "@moh-sso/microfrontend";
import { InlineLoading } from "@carbon/react";
import { PageShell } from "@moh-sso/ui";

import { useListDQATablesQuery } from "./api";

import DataValidationPage from "./pages/data-validation.page";

export function DataValidationRoot(props: MicrofrontendRuntimeProps) {
  void props;
  const tablesQuery = useListDQATablesQuery();

  // Like Report Scheduler: only a loading indicator until the first data is ready.
  if (tablesQuery.isLoading) return <InlineLoading description="Loading data validation…" />;

  return (
    <div className="moh-microfrontend-root">
      <PageShell
        title="Data Validation"
        subtitle="Manage validation rules, run data quality checks, and review flagged records."
      >
        <DataValidationPage />
      </PageShell>
    </div>
  );
}
