import { NextResponse } from "next/server";
import { REFRESH_TOKEN_COOKIE } from "@/lib/session";

// Optimistic only — reads the cookie's presence, never calls the API. Real
// authorization still happens on every Go API call (a missing/expired
// access token gets a real 401 there, refreshed transparently by
// lib/api.js). This just avoids flashing a protected page before redirecting.
const protectedRoutes = ["/account"];
const authRoutes = ["/login", "/register"];

export default function proxy(request) {
  const { pathname } = request.nextUrl;
  const hasSession = request.cookies.has(REFRESH_TOKEN_COOKIE);

  if (protectedRoutes.some((route) => pathname.startsWith(route)) && !hasSession) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  if (authRoutes.includes(pathname) && hasSession) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
