import Link from "next/link";
import Container from "./Container";
import Logo from "./Logo";

const columns = [
  {
    title: "Site",
    links: [
      { href: "/", label: "Home" },
      { href: "/features/", label: "Features" },
      { href: "/about/", label: "About" },
      { href: "/contact/", label: "Contact" },
    ],
  },
  {
    title: "Project",
    links: [
      {
        href: "https://github.com/dprince-03/Bibliomania",
        label: "GitHub",
      },
      {
        href: "https://github.com/dprince-03/Bibliomania/issues",
        label: "Report an issue",
      },
    ],
  },
];

export default function Footer() {
  return (
    <footer className="border-t border-border bg-surface">
      <Container className="grid gap-10 py-14 sm:grid-cols-[1.4fr_1fr_1fr]">
        <div>
          <Logo />
          <p className="mt-4 max-w-xs text-sm text-muted">
            Search, borrow, and read — everything for your next book, in one
            place.
          </p>
        </div>

        {columns.map((col) => (
          <div key={col.title}>
            <h3 className="font-serif text-sm font-semibold text-foreground">
              {col.title}
            </h3>
            <ul className="mt-4 space-y-2.5">
              {col.links.map((link) => (
                <li key={link.href}>
                  <Link
                    href={link.href}
                    target={link.href.startsWith("http") ? "_blank" : undefined}
                    rel={
                      link.href.startsWith("http")
                        ? "noopener noreferrer"
                        : undefined
                    }
                    className="text-sm text-muted transition-colors hover:text-accent"
                  >
                    {link.label}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </Container>

      <div className="border-t border-border py-6">
        <Container>
          <p className="text-xs text-muted">
            © {new Date().getFullYear()} Bibliomania. Built with Go, MySQL,
            Redis, and Next.js.
          </p>
        </Container>
      </div>
    </footer>
  );
}
