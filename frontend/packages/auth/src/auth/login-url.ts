export function currentPortalReturnTo(): string {
  if (typeof window === "undefined") {
    return "/";
  }

  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
}

export function buildLoginURL(loginURL: string, returnTo = currentPortalReturnTo()): string {
  const target = new URL(
    loginURL,
    typeof window === "undefined" ? "http://localhost" : window.location.origin,
  );
  target.searchParams.set("returnTo", returnTo);
  return target.toString();
}
