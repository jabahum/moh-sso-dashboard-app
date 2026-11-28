import type { MicrofrontendRuntimeProps } from "@moh-sso/microfrontend";
import { PageLoading, PageShell } from "@moh-sso/ui";

import { useListDQATablesQuery } from "./api";

import DataValidationPage from "./pages/data-validation.page";

export function DataValidationRoot(props: MicrofrontendRuntimeProps) {
  void props;
  const tablesQuery = useListDQATablesQuery();

  if (tablesQuery.isLoading) return <PageLoading description="Loading data validation…" />;

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
