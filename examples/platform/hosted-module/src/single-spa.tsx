import { createReactMicrofrontendLifecycle } from "@moh-sso/microfrontend";
import { PlatformPilotRoot } from "./root.component";
export const { bootstrap, mount, unmount } = createReactMicrofrontendLifecycle(PlatformPilotRoot);
