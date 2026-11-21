import React, { createContext, useCallback, useContext, useEffect, useState } from "react";

import { ReusableHeaderPanel } from "./ReusableHeaderPanel";

export type PanelSize = "sm" | "md" | "lg";

const PANEL_OPEN_EVENT = "moh:header-panel-open";

function isDesktopDockingEnabled() {
  if (typeof window === "undefined") return true;
  const config = (window as Window & { __APP_CONFIG__?: { sidePanelDocking?: boolean } })
    .__APP_CONFIG__;
  return config?.sidePanelDocking !== false;
}

type HeaderPanelState = {
  dockOnDesktop?: boolean;
  isOpen: boolean;
  title?: string;
  content?: React.ReactNode;
  size: PanelSize;
  primaryActionLabel?: string;
  secondaryActionLabel?: string;
  onPrimaryAction?: () => void;
  onSecondaryAction?: () => void;
  disablePrimaryAction?: boolean;
  maximizable?: boolean;
  defaultMaximized?: boolean;
};

type OpenPanelOptions = {
  dockOnDesktop?: boolean;
  title?: string;
  content: React.ReactNode;
  size?: PanelSize;
  primaryActionLabel?: string;
  secondaryActionLabel?: string;
  onPrimaryAction?: () => void;
  onSecondaryAction?: () => void;
  disablePrimaryAction?: boolean;
  maximizable?: boolean;
  defaultMaximized?: boolean;
};

type HeaderPanelContextType = {
  openPanel: (opts: OpenPanelOptions) => void;
  closePanel: () => void;
};

const HeaderPanelContext = createContext<HeaderPanelContextType | null>(null);

export function HeaderPanelProvider({
  children,
  dockOnDesktop = true,
}: {
  children: React.ReactNode;
  dockOnDesktop?: boolean;
}) {
  const [providerId] = useState(() => Symbol("header-panel-provider"));
  const [state, setState] = useState<HeaderPanelState>({
    isOpen: false,
    size: "md",
  });

  const openPanel = ({
    dockOnDesktop: panelDocking,
    title,
    content,
    size = "md",
    primaryActionLabel,
    secondaryActionLabel,
    onPrimaryAction,
    onSecondaryAction,
    disablePrimaryAction,
    maximizable = true,
    defaultMaximized = false,
  }: OpenPanelOptions) => {
    window.dispatchEvent(new CustomEvent(PANEL_OPEN_EVENT, { detail: providerId }));
    setState({
      isOpen: true,
      dockOnDesktop: panelDocking,
      title,
      content,
      size,
      primaryActionLabel,
      secondaryActionLabel,
      onPrimaryAction,
      onSecondaryAction,
      disablePrimaryAction,
      maximizable,
      defaultMaximized,
    });
  };

  const closePanel = useCallback(() => {
    setState((prev) => (prev.isOpen ? { ...prev, isOpen: false } : prev));
  }, []);

  useEffect(() => {
    const handlePanelOpen = (event: Event) => {
      if ((event as CustomEvent<symbol>).detail !== providerId) closePanel();
    };
    window.addEventListener(PANEL_OPEN_EVENT, handlePanelOpen);
    return () => window.removeEventListener(PANEL_OPEN_EVENT, handlePanelOpen);
  }, [providerId, closePanel]);

  return (
    <HeaderPanelContext.Provider value={{ openPanel, closePanel }}>
      <div className="app-side-panel-scope">
        {children}

        <ReusableHeaderPanel
          isOpen={state.isOpen}
          dockOnDesktop={dockOnDesktop && state.dockOnDesktop !== false && isDesktopDockingEnabled()}
          title={state.title}
          content={state.content}
          size={state.size}
          onClose={closePanel}
          primaryActionLabel={state.primaryActionLabel}
          secondaryActionLabel={state.secondaryActionLabel}
          onPrimaryAction={state.onPrimaryAction}
          onSecondaryAction={state.onSecondaryAction}
          disablePrimaryAction={state.disablePrimaryAction}
          maximizable={state.maximizable ?? true}
          defaultMaximized={state.defaultMaximized ?? false}
        />
      </div>
    </HeaderPanelContext.Provider>
  );
}

export function useHeaderPanel() {
  const ctx = useContext(HeaderPanelContext);
  if (!ctx) {
    throw new Error("useHeaderPanel must be used within HeaderPanelProvider");
  }
  return ctx;
}
