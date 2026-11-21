import { Button } from "@carbon/react";
import { Close, Maximize, Minimize } from "@carbon/react/icons";
import { useEffect, useRef, useState } from "react";

import { useFocusTrap } from "../../accessibility/useFocusTrap";
import type { PanelSize } from "./header-panel.context";
import "./reusable-header-panel.scss";

const DESKTOP_DOCK_QUERY = "(min-width: 80rem)";

type Props = {
  isOpen: boolean;
  dockOnDesktop?: boolean;
  title?: string;
  content?: React.ReactNode;
  onClose: () => void;
  size: PanelSize;
  primaryActionLabel?: string;
  secondaryActionLabel?: string;
  onPrimaryAction?: () => void;
  onSecondaryAction?: () => void;
  disablePrimaryAction?: boolean;
  maximizable?: boolean;
  defaultMaximized?: boolean;
};

export function ReusableHeaderPanel({
  isOpen,
  dockOnDesktop = false,
  title,
  content,
  onClose,
  size,
  primaryActionLabel,
  secondaryActionLabel,
  onPrimaryAction,
  onSecondaryAction,
  disablePrimaryAction = false,
  maximizable = true,
  defaultMaximized = false,
}: Props) {
  const [isMaximized, setIsMaximized] = useState(defaultMaximized);
  const [isDesktop, setIsDesktop] = useState(
    () =>
      dockOnDesktop &&
      typeof window !== "undefined" &&
      window.matchMedia(DESKTOP_DOCK_QUERY).matches,
  );
  const isDocked = dockOnDesktop && isDesktop && !isMaximized;
  const drawerRef = useRef<HTMLElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);

  useFocusTrap({
    active: isOpen,
    trapFocus: !isDocked,
    containerRef: drawerRef,
    initialFocusRef: closeButtonRef,
  });

  useEffect(() => {
    if (!dockOnDesktop) return;

    const media = window.matchMedia(DESKTOP_DOCK_QUERY);
    const update = () => setIsDesktop(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [dockOnDesktop]);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        const target = event.target instanceof Element ? event.target : null;
        const dialog = target?.closest('[role="dialog"], [role="alertdialog"], dialog');
        if (dialog && dialog !== drawerRef.current) return;
        if (
          target?.closest(
            '[role="listbox"], [role="menu"], [role="combobox"][aria-expanded="true"], [aria-haspopup="listbox"][aria-expanded="true"]',
          )
        )
          return;
        event.preventDefault();
        onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown, true);

    return () => document.removeEventListener("keydown", handleKeyDown, true);
  }, [isOpen, onClose]);

  useEffect(() => {
    if (!isOpen || isDocked) return;

    if (!drawerRef.current?.contains(document.activeElement)) {
      closeButtonRef.current?.focus();
    }

    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [isOpen, isDocked]);

  useEffect(() => {
    if (isOpen) {
      setIsMaximized(defaultMaximized);
    }
  }, [isOpen, defaultMaximized]);

  const handleToggleMaximize = () => {
    setIsMaximized((prev) => !prev);
  };

  return (
    <div
      className={`app-side-panel ${isOpen ? "app-side-panel--open" : ""}`}
      data-docked={isDocked || undefined}
      data-size={size}
      aria-hidden={!isOpen}
      inert={!isOpen}
    >
      {!isDocked && (
        <button
          type="button"
          className="app-side-panel__backdrop"
          onClick={onClose}
          aria-label="Close panel backdrop"
          tabIndex={-1}
        />
      )}

      <aside
        ref={drawerRef}
        className={[
          "app-side-panel__drawer",
          `app-side-panel__drawer--${size}`,
          isMaximized ? "app-side-panel__drawer--maximized" : "",
        ].join(" ")}
        role="dialog"
        aria-modal={isDocked ? undefined : true}
        aria-label={title ?? "Panel"}
        tabIndex={-1}
      >
        <div className="app-side-panel__header">
          <h4 className="app-side-panel__title">{title}</h4>

          <div className="app-side-panel__header-actions">
            {maximizable && (
              <Button
                kind="ghost"
                hasIconOnly
                renderIcon={isMaximized ? Minimize : Maximize}
                iconDescription={isMaximized ? "Restore panel size" : "Maximize panel"}
                size="md"
                onClick={handleToggleMaximize}
                tooltipAlignment="end"
              />
            )}

            <Button
              ref={closeButtonRef}
              kind="ghost"
              hasIconOnly
              renderIcon={Close}
              iconDescription="Close panel"
              size="md"
              onClick={onClose}
              tooltipAlignment="end"
            />
          </div>
        </div>

        <div className="app-side-panel__body">{content}</div>

        {(primaryActionLabel || secondaryActionLabel) && (
          <div className="app-side-panel__footer">
            {secondaryActionLabel && (
              <Button
                kind="secondary"
                onClick={onSecondaryAction ?? onClose}
                className="app-side-panel__footer-button"
              >
                {secondaryActionLabel}
              </Button>
            )}

            {primaryActionLabel && (
              <Button
                kind="primary"
                onClick={onPrimaryAction}
                disabled={disablePrimaryAction}
                className="app-side-panel__footer-button"
              >
                {primaryActionLabel}
              </Button>
            )}
          </div>
        )}
      </aside>
    </div>
  );
}
