"use client";

import { useActionState } from "react";
import Link from "next/link";
import Container from "@/components/Container";
import Button from "@/components/Button";
import Input from "@/components/Input";
import { loginAction } from "@/app/actions/auth";

export default function LoginPage() {
  const [state, action, pending] = useActionState(loginAction, undefined);

  return (
    <section className="py-24">
      <Container className="max-w-sm">
        <h1 className="font-serif text-3xl font-semibold text-foreground">
          Sign in
        </h1>
        <form action={action} className="mt-8 space-y-5">
          <Input label="Email" id="email" type="email" required />
          <Input label="Password" id="password" type="password" required />
          {state?.error && <p className="text-sm text-alert">{state.error}</p>}
          <Button type="submit" disabled={pending} className="w-full">
            {pending ? "Signing in…" : "Sign in"}
          </Button>
        </form>
        <p className="mt-6 text-sm text-muted">
          Don&apos;t have an account?{" "}
          <Link href="/register" className="text-link hover:underline">
            Register
          </Link>
        </p>
      </Container>
    </section>
  );
}
