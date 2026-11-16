import { InlineLoading } from "@carbon/react";
import { useMemo } from "react";
import { useParams } from "react-router-dom";
import { useGetApplicationManifestRuntimeQuery } from "@/app/systems/api/application-manifests.api";
import type { MicrofrontendLifecycle } from "@moh-sso/microfrontend";
import { ErrorState } from "@moh-sso/ui";
import { SingleSpaApp } from "./SingleSpaApp";

export function ManifestApplication() {
  const { clientId = "" } = useParams();
  const { data, isFetching, isError, refetch } = useGetApplicationManifestRuntimeQuery(clientId, {
    refetchOnMountOrArgChange: true,
  });
  const entryUrl = data?.launch.entryUrl;
  const loader = useMemo(
    () => async (): Promise<MicrofrontendLifecycle> => {
      if (!entryUrl) throw new Error("The application entry URL is unavailable.");
      const lifecycle: Partial<MicrofrontendLifecycle> = await import(/* @vite-ignore */ entryUrl);
      if (
        typeof lifecycle.bootstrap !== "function" ||
        typeof lifecycle.mount !== "function" ||
        typeof lifecycle.unmount !== "function"
      ) {
        throw new Error("The application must export bootstrap, mount and unmount functions.");
      }
      return lifecycle as MicrofrontendLifecycle;
    },
    [entryUrl],
  );

  if (isFetching) return <InlineLoading description="Checking application access…" />;
  if (
    isError ||
    !data ||
    data.launch.mode !== "microfrontend" ||
    data.launch.route !== `/apps/extensions/${clientId}` ||
    !data.launch.appName
  ) {
    return (
      <ErrorState
        title="Application unavailable"
        description="This application is unavailable or your account does not have access."
        primaryAction={{ label: "Retry", onClick: refetch }}
      />
    );
  }

  return (
    <SingleSpaApp
      key={`${clientId}:${entryUrl}`}
      appName={data.launch.appName}
      basename={data.launch.route}
      lifecycles={loader}
      orchestrate={false}
    />
  );
}
