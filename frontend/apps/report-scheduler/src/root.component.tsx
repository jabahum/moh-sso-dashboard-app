import { Button, InlineLoading, InlineNotification, Tile } from "@carbon/react";
import type { MicrofrontendRuntimeProps } from "@moh-sso/microfrontend";
import { useGetReportSchedulerModuleQuery } from "./api";
import "./report-scheduler.scss";

export function ReportSchedulerRoot(props: MicrofrontendRuntimeProps) {
 void props;
 const { data, isLoading, isFetching, isError, refetch } = useGetReportSchedulerModuleQuery();
 return (
  <div className="moh-microfrontend-root report-scheduler">
   <h1>Report Scheduler</h1>
   <p>Plan recurring report generation and delivery.</p>
   {isLoading && <InlineLoading description="Loading Report Scheduler…" />}
   {isError && (
    <InlineNotification kind="error" title="Unable to load Report Scheduler" subtitle="Try again to connect to the service." hideCloseButton />
   )}
   {data && (
    <Tile>
     <h2>Report scheduling is coming soon</h2>
     <p>Report schedules and delivery options will be available here once scheduling is enabled.</p>
    </Tile>
   )}
   <Button kind="tertiary" size="sm" disabled={isFetching} onClick={() => void refetch()}>Refresh</Button>
  </div>
 );
}
