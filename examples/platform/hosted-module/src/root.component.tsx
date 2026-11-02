import type { MicrofrontendRuntimeProps } from "@moh-sso/microfrontend";
import { useAuthorization } from "@moh-sso/auth";

export function PlatformPilotRoot(props: MicrofrontendRuntimeProps) {
  const { hasSystem } = useAuthorization();
  if (!props.auth?.isAuthenticated || !hasSystem("pilot-hosted")) {
    return <p>Application access is required.</p>;
  }
  return (
    <section className="moh-microfrontend-root" aria-label="Hosted platform pilot">
      <h1>Hosted platform pilot</h1>
      <p>This module uses the shell session. Backend operations enforce their own permissions.</p>
    </section>
  );
}
