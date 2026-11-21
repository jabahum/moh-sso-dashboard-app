import { API } from "@moh-sso/config";
import { authLoaded, loginSuccess, logout as logoutAction } from "../auth/auth.slice";
import type { AuthUser } from "../auth/auth.types";

import { baseApi } from "@moh-sso/api";

type ApiEnvelope<T> = {
  success: boolean;
  data: T;
};

type RefreshResponse = ApiEnvelope<{
  expires_in: number;
}>;

export const authApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    /* -----------------------------
     * Get current user
     * ----------------------------- */
    me: builder.query<AuthUser, void>({
      providesTags: ["Auth"],
      query: () => ({
        url: API.auth.me(),
        method: "GET",
        credentials: "include",
      }),

      transformResponse: (res: ApiEnvelope<{ user: AuthUser }>) => {
        return res.data.user;
      },

      async onQueryStarted(_, { dispatch, queryFulfilled }) {
        try {
          const { data: user } = await queryFulfilled;

          dispatch(
            loginSuccess({
              accessToken: null,
              user,
            }),
          );
        } catch {
          dispatch(logoutAction());
        } finally {
          dispatch(authLoaded());
        }
      },
    }),

    /* -----------------------------
     * Update profile
     * ----------------------------- */
    updateProfile: builder.mutation<AuthUser, { firstName?: string; lastName?: string }>({
      query: (body) => ({
        url: API.auth.me(),
        method: "PUT",
        body,
        credentials: "include",
      }),

      transformResponse: (res: ApiEnvelope<{ user: AuthUser }>) => {
        return res.data.user;
      },
    }),

    /* -----------------------------
     * Refresh session
     *
     * Backend sets HttpOnly cookies.
     * Do not store access_token in Redux.
     * Do not call /me here.
     * ----------------------------- */
    refresh: builder.mutation<RefreshResponse, void>({
      query: () => ({
        url: API.auth.refresh(),
        method: "POST",
        credentials: "include",
      }),
    }),
  }),
});

export const { useMeQuery, useUpdateProfileMutation, useRefreshMutation } = authApi;
