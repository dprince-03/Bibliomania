"use client";

import { useState } from "react";
import Link from "next/link";

// The desktop nav (Navbar.js) hides Catalog/Authors/My borrows below the
// sm breakpoint with no replacement — on a phone there was previously no
// way to reach them at all. This mirrors exactly what desktop already
// shows, rather than inventing a separate mobile-only IA.
export default function MobileMenu({ links, hasSession }) {
  const [open, setOpen] = useState(false);

  return (
    <div className="sm:hidden">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="mobile-menu"
        aria-label={open ? "Close menu" : "Open menu"}
        className="rounded-lg border border-border p-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
      >
        {open ? "✕" : "☰"}
      </button>

      {open && (
        <div
          id="mobile-menu"
          className="absolute inset-x-0 top-16 z-30 border-b border-border bg-background px-6 py-4 shadow-lg"
        >
          <nav className="flex flex-col gap-4">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setOpen(false)}
                className="text-sm font-medium text-foreground"
              >
                {link.label}
              </Link>
            ))}
            {hasSession && (
              <Link
                href="/borrows"
                onClick={() => setOpen(false)}
                className="text-sm font-medium text-foreground"
              >
                My borrows
              </Link>
            )}
            <Link
              href={hasSession ? "/account" : "/login"}
              onClick={() => setOpen(false)}
              className="text-sm font-medium text-accent"
            >
              {hasSession ? "Account" : "Sign in"}
            </Link>
          </nav>
        </div>
      )}
    </div>
  );
}
