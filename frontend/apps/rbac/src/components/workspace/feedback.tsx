import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  type ReactNode,
} from "react";
import { InlineNotification } from "@carbon/react";
import { useModal } from "@moh-sso/ui";

type ConfirmOptions = {
  title: string;
  body: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  danger?: boolean;
};

/**
 * Promise-based confirmation dialog on the shell's modal. Resolves true only
 * when the person chooses the confirm action.
 */
export function useConfirm() {
  const { openModal, closeModal } = useModal();
  return useCallback(
    ({ title, body, confirmLabel, cancelLabel = "Cancel", danger = false }: ConfirmOptions) =>
      new Promise<boolean>((resolve) => {
        let settled = false;
        const settle = (value: boolean) => {
          if (settled) return;
          settled = true;
          resolve(value);
        };
        openModal({
          title,
          size: "sm",
          content: <div className="rbac-confirm">{body}</div>,
          primaryAction: {
            label: confirmLabel,
            kind: danger ? "danger" : "primary",
            onClick: () => {
              settle(true);
              closeModal();
            },
          },
          secondaryAction: {
            label: cancelLabel,
            onClick: () => {
              settle(false);
              closeModal();
            },
          },
          onClose: () => settle(false),
        });
      }),
    [closeModal, openModal],
  );
}

type ActionErrorProps = {
  title: string;
  message?: string | null;
  onClose?: () => void;
  children?: ReactNode;
};

/** An error shown next to the action that failed. */
export function ActionError({ title, message, onClose, children }: ActionErrorProps) {
  if (!message) return null;
  return (
    <div className="rbac-action-error">
      <InlineNotification
        kind="error"
        role="alert"
        lowContrast
        title={title}
        subtitle={message}
        hideCloseButton={!onClose}
        onClose={() => {
          onClose?.();
          return false;
        }}
      />
      {children}
    </div>
  );
}

type UnsavedEntry = { label: string; discard: () => void };
type UnsavedRegistry = Map<string, UnsavedEntry>;

const UnsavedChangesContext = createContext<{ current: UnsavedRegistry } | null>(null);

/** Tracks unsaved edits on the page so navigation can warn before discarding them. */
export function UnsavedChangesProvider({ children }: { children: ReactNode }) {
  const registry = useRef<UnsavedRegistry>(new Map());

  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => {
      if (registry.current.size === 0) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, []);

  return (
    <UnsavedChangesContext.Provider value={registry}>{children}</UnsavedChangesContext.Provider>
  );
}

/** Registers unsaved edits while `dirty` is true. */
export function useUnsavedChanges(dirty: boolean, label: string, discard: () => void) {
  const registry = useContext(UnsavedChangesContext);
  const id = useId();
  const discardRef = useRef(discard);

  useEffect(() => {
    discardRef.current = discard;
  }, [discard]);

  useEffect(() => {
    if (!registry || !dirty) return;
    const entries = registry.current;
    entries.set(id, { label, discard: () => discardRef.current() });
    return () => {
      entries.delete(id);
    };
  }, [dirty, id, label, registry]);
}

/**
 * Returns `guard(proceed)`: runs `proceed` at once when nothing is unsaved,
 * otherwise asks before discarding the unsaved edits.
 */
export function useUnsavedGuard() {
  const registry = useContext(UnsavedChangesContext);
  const confirm = useConfirm();

  return useCallback(
    async (proceed: () => void) => {
      const pending = registry ? [...registry.current.values()] : [];
      if (pending.length === 0) {
        proceed();
        return;
      }
      const discard = await confirm({
        title: "Discard unsaved changes?",
        body: (
          <>
            <p>These changes have not been saved:</p>
            <ul className="rbac-confirm__list">
              {pending.map((entry) => (
                <li key={entry.label}>{entry.label}</li>
              ))}
            </ul>
            <p>Continuing discards them.</p>
          </>
        ),
        confirmLabel: "Discard changes",
        cancelLabel: "Keep editing",
        danger: true,
      });
      if (!discard) return;
      for (const entry of pending) entry.discard();
      proceed();
    },
    [confirm, registry],
  );
}

/** Memoised Set from a list of keys. */
export function useKeySet(keys: string[] | undefined) {
  const joined = (keys ?? []).join("\n");
  return useMemo(() => new Set(joined ? joined.split("\n") : []), [joined]);
}
