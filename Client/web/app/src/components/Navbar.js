import Link from "next/link";
import Container from "./Container";
import Logo from "./Logo";

// Nav links are added as each step's pages land (catalog in Step 3, auth in
// Step 2, etc.) — deliberately empty for now rather than linking to pages
// that don't exist yet.
export default function Navbar() {
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/95 backdrop-blur">
      <Container className="flex h-16 items-center justify-between">
        <Link href="/" className="text-foreground">
          <Logo />
        </Link>
      </Container>
    </header>
  );
}
