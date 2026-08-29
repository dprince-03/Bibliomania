"use server";

import { redirect } from "next/navigation";
import { setSession, clearSession, getRefreshToken } from "@/lib/session";

// Deliberately not routed through lib/api.js: these endpoints don't take
// (or don't yet have) an access token to attach, and a 401 here is a real
// "wrong credentials" answer, not a signal to refresh-and-retry.
const API_INTERNAL_URL = process.env.API_INTERNAL_URL || "http://localhost:8080";

async function callAuthEndpoint(path, body) {
  const res = await fetch(`${API_INTERNAL_URL}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    cache: "no-store",
  });

  const payload = await res.json().catch(() => null);

  if (!res.ok) {
    return { error: payload?.error || "Something went wrong. Please try again." };
  }

  return { data: payload?.data };
}

async function startSessionAndRedirect(authResponse) {
  await setSession({
    accessToken: authResponse.token.access_token,
    refreshToken: authResponse.token.refresh_token,
    expiresIn: authResponse.token.expires_in,
  });
  redirect("/");
}

export async function registerAction(prevState, formData) {
  const firstName = formData.get("first_name")?.toString().trim();
  const lastName = formData.get("last_name")?.toString().trim();
  const email = formData.get("email")?.toString().trim();
  const password = formData.get("password")?.toString();

  if (!firstName || !lastName || !email || !password) {
    return { error: "All fields are required." };
  }

  const { data, error } = await callAuthEndpoint("/api/v1/auth/register", {
    first_name: firstName,
    last_name: lastName,
    email,
    password,
  });

  if (error) return { error };

  await startSessionAndRedirect(data);
}

export async function loginAction(prevState, formData) {
  const email = formData.get("email")?.toString().trim();
  const password = formData.get("password")?.toString();

  if (!email || !password) {
    return { error: "Email and password are required." };
  }

  const { data, error } = await callAuthEndpoint("/api/v1/auth/login", {
    email,
    password,
  });

  if (error) return { error };

  await startSessionAndRedirect(data);
}

export async function logoutAction() {
  const refreshToken = await getRefreshToken();

  if (refreshToken) {
    // Best-effort: revoke server-side, but always clear the local session
    // and redirect even if the API call fails (e.g. already-expired token).
    await callAuthEndpoint("/api/v1/auth/logout", {
      refresh_token: refreshToken,
    }).catch(() => {});
  }

  await clearSession();
  redirect("/login");
}
