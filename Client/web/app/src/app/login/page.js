import LoginForm from "@/components/LoginForm";

// A plain "use client" page can't export metadata — this thin Server
// Component wrapper exists mainly so the browser tab/title reads "Sign in"
// instead of every route sharing the root layout's title.
export const metadata = {
  title: "Sign in — Bibliotheca",
};

export default function LoginPage() {
  return <LoginForm />;
}
