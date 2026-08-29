"use client";

import { useActionState } from "react";
import Link from "next/link";
import Container from "@/components/Container";
import Button from "@/components/Button";
import Input from "@/components/Input";
import { registerAction } from "@/app/actions/auth";

export default function RegisterPage() {
  const [state, action, pending] = useActionState(registerAction, undefined);

  return (
    <section className="py-24">
      <Container className="max-w-sm">
        <h1 className="font-serif text-3xl font-semibold text-foreground">
          Create an account
        </h1>
        <form action={action} className="mt-8 space-y-5">
          <div className="grid grid-cols-2 gap-4">
            <Input label="First name" id="first_name" required minLength={2} />
            <Input label="Last name" id="last_name" required minLength={2} />
          </div>
          <Input label="Email" id="email" type="email" required />
          <Input
            label="Password"
            id="password"
            type="password"
            required
            minLength={8}
          />
          {state?.error && <p className="text-sm text-alert">{state.error}</p>}
          <Button type="submit" disabled={pending} className="w-full">
            {pending ? "Creating account…" : "Create account"}
          </Button>
        </form>
        <p className="mt-6 text-sm text-muted">
          Already have an account?{" "}
          <Link href="/login" className="text-link hover:underline">
            Sign in
          </Link>
        </p>
      </Container>
    </section>
  );
}
