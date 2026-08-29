import { NextResponse } from "next/server";
import { REFRESH_TOKEN_COOKIE } from "@/lib/session";

// Optimistic only — reads the cookie's presence, never calls the API. Real
// authorization still happens on every Go API call (a missing/expired
// access token gets a real 401 there, refreshed transparently by
// lib/api.js). This just avoids flashing a protected page before redirecting.
const protectedRoutes = ["/account", "/borrows", "/library", "/history"];
// /books/{id}/read isn't a static prefix like the routes above, so it gets
// its own pattern rather than an entry in protectedRoutes.
const protectedPatterns = [/^\/books\/[^/]+\/read(\/|$)/];
const authRoutes = ["/login", "/register"];

export default function proxy(request) {
  const { pathname } = request.nextUrl;
  const hasSession = request.cookies.has(REFRESH_TOKEN_COOKIE);
  const isProtected =
    protectedRoutes.some((route) => pathname.startsWith(route)) ||
    protectedPatterns.some((pattern) => pattern.test(pathname));

  if (isProtected && !hasSession) {
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
