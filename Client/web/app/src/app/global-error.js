"use client";

// error.js doesn't cover the root layout itself — only global-error.js does,
// and since it replaces the root layout when triggered, it has to render its
// own <html>/<body>. Kept deliberately plain (no fonts/Navbar/Footer, since
// whatever broke may be in one of those) rather than reusing app styling.
export default function GlobalError({ reset }) {
  return (
    <html lang="en">
      <body style={{ fontFamily: "Georgia, serif", textAlign: "center", padding: "4rem 1.5rem" }}>
        <h1>Something went wrong</h1>
        <p>An unexpected error occurred. Please try again.</p>
        <button type="button" onClick={() => reset()}>
          Try again
        </button>
      </body>
    </html>
  );
}
