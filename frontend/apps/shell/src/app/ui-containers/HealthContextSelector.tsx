import { Select, SelectItem } from "@carbon/react";

import { useHealthContext } from "@moh-sso/auth";
import { useToast } from "@moh-sso/ui";

import "./health-context-selector.scss";

export function HealthContextSelector() {
  const { contexts, activeContext, isLoading, isUpdating, selectContext } = useHealthContext();
  const toast = useToast();
  const hasMultipleContexts = contexts.length > 1;
  const value = activeContext?.id ?? "";

  // Nothing to show or choose: don't spend header space on a disabled
  // "No health context assigned" control (or a transient loading state).
  if (isLoading || contexts.length === 0) {
    return null;
  }

  return (
    <div className="health-context-selector">
      <Select
        id="global-health-context"
        labelText="Health context"
        hideLabel
        size="sm"
        value={value}
        disabled={isUpdating || !hasMultipleContexts}
        onChange={(event) => {
          void selectContext(event.target.value).catch(() => {
            toast.error("Context change failed", "Your previous health context remains active.");
          });
        }}
      >
        {contexts.map((context) => (
          <SelectItem
            key={context.id}
            value={context.id}
            text={`${context.name} (${context.contextType.replaceAll("_", " ").toLowerCase()})`}
          />
        ))}
      </Select>
    </div>
  );
}
