"use server";

import { revalidatePath } from "next/cache";
import { apiPatch, ApiError } from "@/lib/api";

// Sending "" rather than omitting/nulling an untouched field matters here:
// the Go API treats a present-but-empty string as "clear this field" and a
// missing/null one as "leave it alone" (UpdateProfileRequest's fields are
// *string). Since this form is always pre-filled with the current values,
// an unedited field round-trips its existing value anyway — only a
// deliberately-cleared field ends up empty, which is exactly when clearing
// it server-side is the right behavior.
export async function updateProfileAction(prevState, formData) {
  const phoneNumber = formData.get("phone_number")?.toString() ?? "";
  const bio = formData.get("bio")?.toString() ?? "";
  const profilePicture = formData.get("profile_picture")?.toString() ?? "";

  try {
    await apiPatch("/api/v1/users/me", {
      phone_number: phoneNumber,
      bio,
      profile_picture: profilePicture,
    });
  } catch (err) {
    if (err instanceof ApiError) {
      return { error: err.message };
    }
    return { error: "Could not update your profile. Please try again." };
  }

  revalidatePath("/account");
  return { success: true };
}
