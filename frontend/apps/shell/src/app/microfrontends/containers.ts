const containers = new Map<string, HTMLElement>();

export function microfrontendContainerId(appName: string) {
  return `single-spa-application-${appName.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
}

export function getMicrofrontendContainer(appName: string) {
  const cachedContainer = containers.get(appName);
  if (cachedContainer) {
    return cachedContainer;
  }

  const id = microfrontendContainerId(appName);
  const container = document.getElementById(id) ?? document.createElement("div");
  container.id = id;
  container.dataset.microfrontend = appName;
  // single-spa can bootstrap before the shell commits its route. Keep the
  // container detached until that route attaches it inside the content area.
  containers.set(appName, container);
  return container;
}

export function attachMicrofrontendContainer(appName: string, host: HTMLElement) {
  const container = getMicrofrontendContainer(appName);
  if (container.parentElement !== host) {
    host.appendChild(container);
  }
  return container;
}
