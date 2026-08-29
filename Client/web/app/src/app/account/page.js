import Container from "@/components/Container";
import Button from "@/components/Button";
import { apiGet, ApiError } from "@/lib/api";
import { logoutAction } from "@/app/actions/auth";

// Minimal placeholder proving proxy.js + lib/api.js's Authorization
// attachment work end-to-end. The real profile UI (edit, personal
// library, reading history) is Step 5 — this page gets replaced there.
export default async function AccountPage() {
  let user = null;
  let error = null;

  try {
    user = await apiGet("/api/v1/users/me");
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
          <dl className="mt-6 space-y-3 text-sm">
            <div>
              <dt className="text-muted">Name</dt>
              <dd className="text-foreground">
                {user.first_name} {user.last_name}
              </dd>
            </div>
            <div>
              <dt className="text-muted">Email</dt>
              <dd className="text-foreground">{user.email}</dd>
            </div>
            <div>
              <dt className="text-muted">Role</dt>
              <dd className="text-foreground">{user.role}</dd>
            </div>
          </dl>
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
