import { createReactMicrofrontendLifecycle } from "@moh-sso/microfrontend";
import { ReportSchedulerRoot } from "./root.component";

export const { bootstrap, mount, unmount } = createReactMicrofrontendLifecycle(ReportSchedulerRoot);
