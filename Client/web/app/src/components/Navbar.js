import Link from "next/link";
import Container from "./Container";
import Logo from "./Logo";
import Button from "./Button";
import { getRefreshToken } from "@/lib/session";

// Nav links beyond auth are added as each step's pages land (catalog in
// Step 3, etc.) — deliberately minimal for now.
export default async function Navbar() {
  const hasSession = Boolean(await getRefreshToken());

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/95 backdrop-blur">
      <Container className="flex h-16 items-center justify-between">
        <Link href="/" className="text-foreground">
          <Logo />
        </Link>

        {hasSession ? (
          <Button href="/account" variant="ghost">
            Account
          </Button>
        ) : (
          <Button href="/login" variant="ghost">
            Sign in
          </Button>
        )}
      </Container>
    </header>
  );
}
