import {
  createApi,
  fetchBaseQuery,
  type BaseQueryFn,
  type FetchArgs,
  type FetchBaseQueryError,
} from "@reduxjs/toolkit/query/react";

import { API } from "@moh-sso/config";
import { loginSuccess, logout } from "../../../auth/src/auth/auth.slice";
import type { AuthUser } from "../../../auth/src/auth/auth.types";

type ApiEnvelope<T> = {
  success: boolean;
  data: T;
};

type MeResponse = ApiEnvelope<{
  user: AuthUser;
}>;

/**
 * Raw base query
 *
 * Important:
 * Since tokens are now stored in HttpOnly cookies,
 * we do NOT attach Authorization: Bearer token manually.
 *
 * Browser sends cookies automatically because credentials is "include".
 */
const rawBaseQuery = fetchBaseQuery({
  baseUrl: API.base,
  credentials: "include",
});

/**
 * Single-flight refresh promise.
 *
 * This prevents multiple simultaneous refresh requests,
 * which can break Keycloak refresh-token rotation.
 */
let refreshPromise: Promise<boolean> | null = null;

function getUrl(args: string | FetchArgs): string {
  if (typeof args === "string") {
    return args;
  }

  return String(args.url);
}

function isAuthRefreshRequest(args: string | FetchArgs): boolean {
  return getUrl(args).includes("/auth/refresh");
}

function isAuthLoginRequest(args: string | FetchArgs): boolean {
  return getUrl(args).includes("/auth/login");
}

function isAuthLogoutRequest(args: string | FetchArgs): boolean {
  return getUrl(args).includes("/auth/logout");
}

function isAuthCallbackRequest(args: string | FetchArgs): boolean {
  return getUrl(args).includes("/auth/callback");
}

function isAuthMeRequest(args: string | FetchArgs): boolean {
  return getUrl(args).includes("/auth/me");
}

function shouldSkipRefresh(args: string | FetchArgs): boolean {
  return (
    isAuthRefreshRequest(args) ||
    isAuthLoginRequest(args) ||
    isAuthLogoutRequest(args) ||
    isAuthCallbackRequest(args)
  );
}

/**
 * Base query with safe 401 refresh handling.
 */
const baseQueryWithReauth: BaseQueryFn<string | FetchArgs, unknown, FetchBaseQueryError> = async (
  args,
  api,
  extraOptions,
) => {
  let result = await rawBaseQuery(args, api, extraOptions);

  /**
   * If the request succeeded or failed for anything other than 401,
   * just return the result.
   */
  if (result.error?.status !== 401) {
    return result;
  }

  /**
   * Never refresh when the failed request itself is auth refresh/login/logout/callback.
   * This prevents infinite loops.
   */
  if (shouldSkipRefresh(args)) {
    api.dispatch(logout());
    return result;
  }

  /**
   * Send only one refresh request even if many API calls failed with 401.
   */
  if (!refreshPromise) {
    refreshPromise = (async () => {
      try {
        const refreshResult = await rawBaseQuery(
          {
            url: "/auth/refresh",
            method: "POST",
            credentials: "include",
          },
          api,
          extraOptions,
        );

        return !refreshResult.error;
      } catch {
        return false;
      } finally {
        refreshPromise = null;
      }
    })();
  }

  const refreshed = await refreshPromise;

  if (!refreshed) {
    api.dispatch(logout());
    return result;
  }

  /**
   * Retry the original request after backend sets new HttpOnly cookies.
   */
  result = await rawBaseQuery(args, api, extraOptions);

  // A successful refresh does not guarantee the session is usable. Clear auth
  // if the retry is still unauthorized so ProtectedRoute sends the user to login.
  if (result.error?.status === 401) {
    api.dispatch(logout());
    return result;
  }

  /**
   * If this was /auth/me and it succeeded, sync Redux auth state.
   */
  if (!result.error && isAuthMeRequest(args)) {
    const envelope = result.data as MeResponse | undefined;
    const user = envelope?.data?.user;

    if (user) {
      api.dispatch(
        loginSuccess({
          accessToken: null,
          user,
        }),
      );
    }
  }

  return result;
};

export const baseApi = createApi({
  reducerPath: "api",
  baseQuery: baseQueryWithReauth,

  tagTypes: [
    "Auth",
    "User",
    "Client",
    "Audit",
    "Metrics",
    "Notification",
    "ClientRole",
    "UserClientRole",
    "Document",
    "Documents",
    "DocumentProcesses",
    "StorageLocations",
    "StorageLocation",
    "UserSession",
    "UserSessions",
    "Announcements",
    "ThemeElement",
    "Surveillance",
    "GeoJson",
    "Issues",
    "Transactions",
    "Emails",
    "DocumentTemplates",
    "DocumentSheets",
    "DocumentColumns",
    "RbacSystem",
    "RbacGroup",
    "RbacPermission",
    "RbacRealmRole",
    "RbacAccessRequest",
    "RbacChangeRequest",
    "RbacAudit",
    "Datasets",
    "DataElements",
    "DQATables",
    "DQARules",
    "DQARuns",
    "DQASchedules",
    "DQAFlags",
  ],

  endpoints: () => ({}),
});
