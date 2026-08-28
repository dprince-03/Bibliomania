"use client";

import { useState } from "react";
import Link from "next/link";
import Container from "./Container";
import Logo from "./Logo";
import Button from "./Button";

const links = [
  { href: "/", label: "Home" },
  { href: "/features/", label: "Features" },
  { href: "/about/", label: "About" },
  { href: "/contact/", label: "Contact" },
];

const appUrl = process.env.NEXT_PUBLIC_APP_URL || "http://app.bibliotheca.local";

export default function Navbar() {
  const [open, setOpen] = useState(false);

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/90 backdrop-blur">
      <Container className="flex h-16 items-center justify-between">
        <Link href="/" className="text-foreground">
          <Logo />
        </Link>

        <nav className="hidden items-center gap-8 md:flex">
          {links.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="text-sm font-medium text-muted transition-colors hover:text-accent"
            >
              {link.label}
            </Link>
          ))}
        </nav>

        <div className="hidden md:block">
          <Button href={appUrl}>Open the App</Button>
        </div>

        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-label="Toggle menu"
          aria-expanded={open}
          className="flex h-9 w-9 items-center justify-center rounded-full border border-border text-foreground md:hidden"
        >
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
            {open ? (
              <path
                d="M3 3l12 12M15 3L3 15"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
              />
            ) : (
              <path
                d="M2 4.5h14M2 9h14M2 13.5h14"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
              />
            )}
          </svg>
        </button>
      </Container>

      {open && (
        <div className="border-t border-border bg-background md:hidden">
          <Container className="flex flex-col gap-4 py-5">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setOpen(false)}
                className="text-sm font-medium text-muted hover:text-accent"
              >
                {link.label}
              </Link>
            ))}
            <Button href={appUrl} className="mt-1 w-fit">
              Open the App
            </Button>
          </Container>
        </div>
      )}
    </header>
  );
}
