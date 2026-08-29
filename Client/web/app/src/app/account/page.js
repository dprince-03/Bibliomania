import Link from "next/link";
import Container from "@/components/Container";
import Button from "@/components/Button";
import ProfileForm from "@/components/ProfileForm";
import { apiGet, ApiError } from "@/lib/api";
import { logoutAction } from "@/app/actions/auth";

export const metadata = {
  title: "Your account — Bibliomania",
};

const quickLinks = [
  { href: "/borrows", label: "My borrows" },
  { href: "/library", label: "My library" },
  { href: "/history", label: "Reading history" },
];

export default async function AccountPage() {
  let profile = null;
  let error = null;

  try {
    profile = await apiGet("/api/v1/users/me");
  } catch (err) {
    error = err instanceof ApiError ? err.message : "Could not load your account.";
  }

  return (
    <section className="py-24">
      <Container className="max-w-sm">
        <h1 className="font-serif text-3xl font-semibold text-foreground">
          Your account
        </h1>

        {error ? (
          <p className="mt-6 text-sm text-alert">{error}</p>
        ) : (
          <>
            <dl className="mt-6 space-y-3 text-sm">
              <div>
                <dt className="text-muted">Name</dt>
                <dd className="text-foreground">
                  {profile.first_name} {profile.last_name}
                </dd>
              </div>
              <div>
                <dt className="text-muted">Email</dt>
                <dd className="text-foreground">{profile.email}</dd>
              </div>
              <div>
                <dt className="text-muted">Role</dt>
                <dd className="text-foreground">{profile.role}</dd>
              </div>
              <div>
                <dt className="text-muted">Reading</dt>
                <dd className="text-foreground">
                  {profile.total_books_read} book
                  {profile.total_books_read === 1 ? "" : "s"} ·{" "}
                  {profile.total_pages_read.toLocaleString()} pages
                </dd>
              </div>
            </dl>

            <ProfileForm profile={profile} />

            <nav className="mt-10 flex flex-col gap-2 border-t border-border pt-6 text-sm">
              {quickLinks.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className="text-link hover:underline"
                >
                  {link.label} →
                </Link>
              ))}
            </nav>
          </>
        )}

        <form action={logoutAction} className="mt-8">
          <Button type="submit" variant="ghost">
            Log out
          </Button>
        </form>
      </Container>
    </section>
  );
}
