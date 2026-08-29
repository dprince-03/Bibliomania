import Link from "next/link";
import Container from "./Container";
import Logo from "./Logo";
import Button from "./Button";
import MobileMenu from "./MobileMenu";
import { getRefreshToken } from "@/lib/session";

const links = [
  { href: "/", label: "Catalog" },
  { href: "/authors", label: "Authors" },
];

export default async function Navbar() {
  const hasSession = Boolean(await getRefreshToken());

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/95 backdrop-blur">
      <Container className="flex h-16 items-center justify-between">
        <Link href="/" className="text-foreground">
          <Logo />
        </Link>

        <nav className="hidden items-center gap-6 sm:flex">
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

        <div className="hidden items-center gap-3 sm:flex">
          {hasSession && (
            <Link
              href="/borrows"
              className="text-sm font-medium text-muted transition-colors hover:text-accent"
            >
              My borrows
            </Link>
          )}
          <Button href={hasSession ? "/account" : "/login"} variant="ghost">
            {hasSession ? "Account" : "Sign in"}
          </Button>
        </div>

        <MobileMenu links={links} hasSession={hasSession} />
      </Container>
    </header>
  );
}
