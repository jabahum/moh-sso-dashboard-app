import { useEffect, type RefObject } from "react";

type FocusTrapOptions = {
  active: boolean;
  trapFocus?: boolean;
  containerRef: RefObject<HTMLElement | null>;
  restoreFocus?: boolean;
  initialFocusRef?: RefObject<HTMLElement | HTMLButtonElement | null>;
  initialFocusSelector?: string;
};

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "textarea:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function getFocusableElements(container: HTMLElement) {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (element) =>
      !element.hasAttribute("disabled") &&
      !element.closest("[inert], [aria-hidden='true']") &&
      element.getClientRects().length > 0,
  );
}

export function useFocusTrap({
  active,
  trapFocus = true,
  containerRef,
  restoreFocus = true,
  initialFocusRef,
  initialFocusSelector,
}: FocusTrapOptions) {
  useEffect(() => {
    if (!active) {
      return;
    }

    const container = containerRef.current;
    const previouslyFocused =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;

    if (!container) {
      return;
    }

    const focusInitialElement = () => {
      const initialElement =
        initialFocusRef?.current ??
        (initialFocusSelector ? container.querySelector<HTMLElement>(initialFocusSelector) : null);

      if (initialElement) {
        initialElement.focus();
        return;
      }

      const focusableElements = getFocusableElements(container);
      (focusableElements[0] ?? container).focus();
    };

    const focusFrame = requestAnimationFrame(focusInitialElement);

    return () => {
      cancelAnimationFrame(focusFrame);
      if (restoreFocus && previouslyFocused?.isConnected) {
        previouslyFocused.focus();
      }
    };
  }, [active, containerRef, initialFocusRef, initialFocusSelector, restoreFocus]);

  useEffect(() => {
    const container = containerRef.current;
    if (!active || !trapFocus || !container) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Tab") {
        return;
      }

      const focusableElements = getFocusableElements(container);

      if (focusableElements.length === 0) {
        event.preventDefault();
        container.focus();
        return;
      }

      const firstElement = focusableElements[0];
      const lastElement = focusableElements[focusableElements.length - 1];
      const activeElement = document.activeElement;

      if (event.shiftKey && activeElement === firstElement) {
        event.preventDefault();
        lastElement.focus();
        return;
      }

      if (!event.shiftKey && activeElement === lastElement) {
        event.preventDefault();
        firstElement.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [active, containerRef, trapFocus]);
}
