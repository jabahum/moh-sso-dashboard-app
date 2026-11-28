import { InlineLoading } from "@carbon/react";
import { useEffect, useState } from "react";

type PageLoadingProps = {
  description?: string;
};

/**
 * What an app renders instead of its page until its first data load finishes.
 * Apps return it before their PageShell, so the page appears once, complete.
 */
export function PageLoading({ description = "Loading…" }: PageLoadingProps) {
  return <InlineLoading className="moh-page-loading" description={description} />;
}

/**
 * True until `isLoading` first turns false, then false for the rest of the
 * component's life. Use it when a later query (a new filter, say) starts a
 * fresh first load that must not swap the page back to PageLoading.
 */
export function useFirstLoad(isLoading: boolean): boolean {
  const [done, setDone] = useState(!isLoading);
  useEffect(() => {
    if (!isLoading) setDone(true);
  }, [isLoading]);
  return !done && isLoading;
}
