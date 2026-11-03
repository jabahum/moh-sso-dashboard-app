import { useState } from "react";
import { Content } from "@carbon/react";
import "@carbon/styles/css/styles.css";
import { ExternalAppHeader } from "./ExternalAppHeader";

export default { title: "Shell/External application header" };

export const Logistics = () => {
  const [action, setAction] = useState("Choose a header action to preview it.");

  return (
    <>
      <ExternalAppHeader
        appName="Logistics Management"
        username="pilot.user"
        portalUrl="https://dashboards.health.go.ug/portal/apps"
        applications={[
          {
            id: "portal",
            name: "Integrated Health Portal",
            href: "https://dashboards.health.go.ug/portal/apps",
          },
          { id: "reports", name: "Example Reports", href: "https://reports.example.org" },
        ]}
        onProfile={() => setAction("Profile action selected; the host app opens its profile page.")}
        onLogout={() =>
          setAction("Logout selected; the host app ends its own session and signs out.")
        }
      />
      <Content id="main-content">
        <h1>Logistics Management</h1>
        <p>This app runs on its own origin and reuses the MOH header.</p>
        <p role="status">{action}</p>
      </Content>
    </>
  );
};
