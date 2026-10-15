import { PERMISSIONS, type Permission } from "@moh-sso/auth";

// Match backend route permissions. Manage broadens ownership scope only;
// it never replaces an action permission or the parent system requirement.
export function schedulerPermissions(can: (permission: Permission) => boolean, hasSystem: boolean) {
  const read = hasSystem && can(PERMISSIONS.reportSchedulerRead);
  return {
    read,
    create: read && can(PERMISSIONS.reportSchedulerCreate),
    update: read && can(PERMISSIONS.reportSchedulerUpdate),
    delete: read && can(PERMISSIONS.reportSchedulerDelete),
    execute: read && can(PERMISSIONS.reportSchedulerExecute),
    history: read && can(PERMISSIONS.reportSchedulerHistory),
    manage: read && can(PERMISSIONS.reportSchedulerManage),
  };
}
export type SchedulerPermissions = ReturnType<typeof schedulerPermissions>;
export function canScheduleAction(access: SchedulerPermissions, action: string): boolean {
  switch (action) {
    case "run":
      return access.execute;
    case "pause":
    case "resume":
      return access.update;
    case "duplicate":
      return access.create;
    case "delete":
      return access.delete;
    default:
      return false;
  }
}
