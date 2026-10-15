import { useEffect, useRef } from "react";
import { useToast } from "@moh-sso/ui";

type QueryNotice = {
  id: string;
  active: boolean;
  kind: "error" | "warning";
  title: string;
  subtitle?: string;
};

// Polling and unrelated renders must not repeat the same outstanding warning.
// Recovering clears its ID so a later failure can notify the user again.
export function useSchedulerQueryToasts(notices: QueryNotice[]) {
  const toast = useToast();
  const previous = useRef(new Set<string>());

  useEffect(() => {
    const active = new Set<string>();
    for (const notice of notices) {
      if (!notice.active) continue;
      active.add(notice.id);
      if (!previous.current.has(notice.id)) {
        toast[notice.kind]({ title: notice.title, subtitle: notice.subtitle });
      }
    }
    previous.current = active;
  }, [notices, toast]);
}
