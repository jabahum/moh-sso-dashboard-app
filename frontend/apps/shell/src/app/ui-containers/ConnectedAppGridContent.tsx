import { useEffect, useMemo } from "react";
import { useDispatch } from "react-redux";
import { useNavigate } from "react-router-dom";

import { resolveSystemLaunchUrl, useAuthorization } from "@moh-sso/auth";
import { API } from "@moh-sso/config";
import { AppGridContent } from "@moh-sso/ui";
import { setActiveClient, setClients } from "@moh-sso/state";

import {
  buildAccessibleLauncherEntries,
  buildAccessibleSideNavClients,
} from "@/app/access/accessClients";

type ConnectedAppGridContentProps = {
  onSelect?: () => void;
};

function isExternalUrl(url: string): boolean {
  return /^https?:\/\//i.test(url);
}

function normalizePortalPath(href: string): string {
  if (isExternalUrl(href)) {
    return href;
  }

  // The portal's own launch URL ("/", or legacy "/portal") opens the app catalogue.
  if (href === "/" || href === "/portal") {
    return "/apps";
  }

  // Stored legacy /portal/... paths map onto the root-served shell.
  if (href.startsWith("/portal/")) {
    return href.replace(/^\/portal/, "");
  }

  return href;
}

export function ConnectedAppGridContent({ onSelect }: ConnectedAppGridContentProps) {
  const dispatch = useDispatch();
  const navigate = useNavigate();
  const { accessibleSystems, can } = useAuthorization();
  const visibleClients = useMemo(
    () =>
      buildAccessibleLauncherEntries({
        accessibleSystems,
        can: (permission) => can(permission as never),
      }),
    [accessibleSystems, can],
  );
  const stateClients = useMemo(() => {
    const clients = buildAccessibleSideNavClients({ accessibleSystems });
    const clientIds = new Set(clients.map((client) => client.clientId));

    for (const client of visibleClients) {
      if (!clientIds.has(client.clientId)) {
        clients.push(client);
      }
    }

    return clients;
  }, [accessibleSystems, visibleClients]);

  useEffect(() => {
    dispatch(setClients(stateClients));
  }, [dispatch, stateClients]);

  const handleOpenClient = (href: string, clientId?: string) => {
    const client = visibleClients.find((item) => item.clientId === clientId);
    const parentClientId = client?.attributes?.["ui.parentClientId"] || clientId;

    if (parentClientId) {
      dispatch(setActiveClient(parentClientId));
    }

    const launchMode = client?.attributes?.["ui.launchMode"] ?? "internal";
    const targetHref = normalizePortalPath(
      resolveSystemLaunchUrl(
        {
          launchUrl: href,
          authenticatedLaunchUrl: client?.attributes?.["ui.authenticatedLaunchUrl"],
        },
        { apiBaseUrl: API.serviceBase, hasPortalSession: true },
      ),
    );

    if (launchMode === "new_tab") {
      window.open(targetHref, "_blank", "noopener,noreferrer");
      return;
    }

    if (launchMode === "same_tab") {
      window.location.assign(targetHref);
      return;
    }

    navigate(targetHref);
  };

  return (
    <AppGridContent clients={visibleClients} onSelect={onSelect} onOpenClient={handleOpenClient} />
  );
}
