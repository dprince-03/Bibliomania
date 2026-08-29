"use client";

import { useActionState } from "react";
import Button from "./Button";
import Input from "./Input";
import { updateProfileAction } from "@/app/actions/profile";

export default function ProfileForm({ profile }) {
  const [state, action, pending] = useActionState(updateProfileAction, undefined);

  return (
    <form action={action} className="mt-8 space-y-5">
      <Input
        label="Phone number"
        id="phone_number"
        defaultValue={profile.phone_number ?? ""}
        minLength={7}
        maxLength={20}
      />

      <div>
        <label htmlFor="bio" className="block text-sm font-medium text-foreground">
          Bio
        </label>
        <textarea
          id="bio"
          name="bio"
          rows={4}
          maxLength={500}
          defaultValue={profile.bio ?? ""}
          className="mt-1.5 w-full rounded-lg border border-border bg-background px-3.5 py-2.5 text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
        />
      </div>

      <Input
        label="Profile picture URL"
        id="profile_picture"
        type="url"
        defaultValue={profile.profile_picture ?? ""}
      />

      {state?.error && <p className="text-sm text-alert">{state.error}</p>}
      {state?.success && <p className="text-sm text-muted">Saved.</p>}

      <Button type="submit" disabled={pending}>
        {pending ? "Saving…" : "Save changes"}
      </Button>
    </form>
  );
}
