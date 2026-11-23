import { useEffect, useState } from "react";
import {
  Button,
  InlineNotification,
  ProgressIndicator,
  ProgressStep,
  Tag,
  TextArea,
} from "@carbon/react";
import { useToast } from "@moh-sso/ui";

import { useTransitionRbacSystemMutation } from "../../api";
import type { ApplicationLifecycleStatus, RbacSystemDetail } from "../../types";
import { ActionError, useConfirm } from "./feedback";
import { getApiErrorMessage, lifecycleLabels, lifecycleTagType } from "./workspace-helpers";

const lifecycleActions: Record<ApplicationLifecycleStatus, ApplicationLifecycleStatus[]> = {
  registered: ["configured", "retired"],
  configured: ["validated", "retired"],
  validated: ["approved", "configured", "retired"],
  approved: ["active", "configured", "retired"],
  active: ["suspended", "retired"],
  suspended: ["configured", "retired"],
  retired: [],
};

const actionCopy: Record<
  ApplicationLifecycleStatus,
  { label: string; description: string; destructive?: boolean }
> = {
  registered: { label: "Register", description: "" },
  configured: {
    label: "Mark as configured",
    description: "Confirms the saved settings are complete. Moving back here restarts review.",
  },
  validated: {
    label: "Validate",
    description:
      "Checks ownership, support, launch settings, the integration contract and enabled access roles.",
  },
  approved: { label: "Approve", description: "Approves the validated configuration for launch." },
  active: {
    label: "Activate",
    description: "Makes the application available to users who hold an access role.",
  },
  suspended: {
    label: "Suspend",
    description: "Hides the application from users until it is configured and approved again.",
    destructive: true,
  },
  retired: {
    label: "Retire",
    description: "Permanently retires the application. No further status changes are possible.",
    destructive: true,
  },
};

const steps: ApplicationLifecycleStatus[] = [
  "registered",
  "configured",
  "validated",
  "approved",
  "active",
];

const narrowQuery = "(max-width: 672px)";

/** True on narrow screens, where the progress steps stack vertically. */
function useNarrowScreen() {
  const [narrow, setNarrow] = useState(
    () => typeof window !== "undefined" && window.matchMedia?.(narrowQuery).matches,
  );
  useEffect(() => {
    const media = window.matchMedia?.(narrowQuery);
    if (!media) return;
    const update = () => setNarrow(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  return narrow;
}

type LifecyclePanelProps = {
  system: RbacSystemDetail;
  canWriteSystems: boolean;
  canWriteRoles: boolean;
};

export function LifecyclePanel({ system, canWriteSystems, canWriteRoles }: LifecyclePanelProps) {
  const toast = useToast();
  const confirm = useConfirm();
  const [transitionSystem, { isLoading: transitioning }] = useTransitionRbacSystemMutation();
  const narrow = useNarrowScreen();
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);

  const status = system.lifecycleStatus;
  const actions = lifecycleActions[status] ?? [];
  const reasonMissing = !reason.trim();
  // Suspended applications were active; show the progress they reached.
  const currentIndex = status === "suspended" ? steps.length - 1 : steps.indexOf(status);

  const needsRolesWrite = (target: ApplicationLifecycleStatus) =>
    target === "approved" || target === "active";
  const isDisabled = (target: ApplicationLifecycleStatus) =>
    !canWriteSystems ||
    transitioning ||
    reasonMissing ||
    (needsRolesWrite(target) && !canWriteRoles);

  const handleTransition = async (target: ApplicationLifecycleStatus) => {
    if (isDisabled(target)) return;
    const copy = actionCopy[target];
    if (copy.destructive) {
      const confirmed = await confirm({
        title: `${copy.label} ${system.displayName}?`,
        body: (
          <>
            <p>{copy.description}</p>
            <p>
              Reason: <strong>{reason.trim()}</strong>
            </p>
          </>
        ),
        confirmLabel: copy.label,
        danger: true,
      });
      if (!confirmed) return;
    }
    try {
      setError(null);
      await transitionSystem({
        clientId: system.clientId,
        status: target,
        reason: reason.trim(),
      }).unwrap();
      setReason("");
      toast.success("Status changed", `${system.displayName} is now ${lifecycleLabels[target]}.`);
    } catch (failure) {
      setError(
        getApiErrorMessage(
          failure,
          "Unable to change application status. Check saved settings, access roles, and permissions.",
        ),
      );
    }
  };

  const blockers: string[] = [];
  if (!canWriteSystems) blockers.push("Changing status requires the rbac:write permission.");
  else if (reasonMissing && actions.length) blockers.push("Enter a reason to enable the actions.");
  if (canWriteSystems && !canWriteRoles && actions.some(needsRolesWrite))
    blockers.push("Approve and Activate also require the rbac:roles:write permission.");

  return (
    <section className="rbac-card" aria-labelledby="rbac-lifecycle-title">
      <div className="rbac-card__header">
        <div>
          <h2 id="rbac-lifecycle-title">Lifecycle</h2>
          <p>
            Applications move through review before users can launch them. Save settings before
            changing status.
          </p>
        </div>
        <Tag type={lifecycleTagType[status]} size="md">
          {lifecycleLabels[status]}
        </Tag>
      </div>
      <div className="rbac-card__body">
        <ProgressIndicator
          currentIndex={currentIndex < 0 ? 0 : currentIndex}
          spaceEqually={!narrow}
          vertical={narrow}
          className="rbac-lifecycle-progress"
        >
          {steps.map((step) => (
            <ProgressStep
              key={step}
              label={
                step === "active" && status === "suspended" ? "Suspended" : lifecycleLabels[step]
              }
              invalid={step === "active" && status === "suspended"}
              disabled={status === "retired"}
              complete={step === "active" && status === "active"}
            />
          ))}
        </ProgressIndicator>

        {status === "suspended" && (
          <InlineNotification
            kind="warning"
            lowContrast
            hideCloseButton
            title="Suspended"
            subtitle="Users cannot launch this application. Mark it as configured to start review again."
          />
        )}
        {status === "retired" && (
          <InlineNotification
            kind="info"
            lowContrast
            hideCloseButton
            title="Retired"
            subtitle="This application is retired. No further status changes are possible."
          />
        )}
        <p className="rbac-hint">
          Changing ownership, launch settings, environment, navigation or integration sends the
          application back for validation and approval.
        </p>

        {actions.length > 0 && (
          <div className="rbac-lifecycle-form">
            <TextArea
              id="application-lifecycle-reason"
              labelText="Reason for status change (required)"
              helperText="Recorded in the audit trail."
              rows={2}
              value={reason}
              maxLength={2000}
              disabled={!canWriteSystems || transitioning}
              onChange={(event) => setReason(event.target.value)}
            />
            <div className="rbac-lifecycle-actions">
              {actions.map((target) => (
                <div className="rbac-lifecycle-action" key={target}>
                  <Button
                    kind={
                      actionCopy[target].destructive
                        ? "danger--tertiary"
                        : target === actions[0]
                          ? "primary"
                          : "tertiary"
                    }
                    size="md"
                    disabled={isDisabled(target)}
                    onClick={() => void handleTransition(target)}
                  >
                    {actionCopy[target].label}
                  </Button>
                  <small>{actionCopy[target].description}</small>
                </div>
              ))}
            </div>
            {blockers.length > 0 && (
              <ul className="rbac-form-problems" aria-label="Why actions are unavailable">
                {blockers.map((blocker) => (
                  <li key={blocker}>{blocker}</li>
                ))}
              </ul>
            )}
          </div>
        )}
        <ActionError title="Status not changed" message={error} onClose={() => setError(null)} />
      </div>
    </section>
  );
}
