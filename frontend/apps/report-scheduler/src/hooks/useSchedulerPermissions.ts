import { useAuthorization } from "@moh-sso/auth";
import { schedulerPermissions } from "../utils/scheduler-permissions";

export function useSchedulerPermissions() {
  const { can, hasSystem } = useAuthorization();
  return schedulerPermissions(can, hasSystem("data-statistics"));
}
