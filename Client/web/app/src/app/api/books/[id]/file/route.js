// Proxies GET /api/v1/books/{id}/download from the Go API, attaching the
// Authorization header the browser can't add itself (the access token lives
// in an httpOnly cookie). Two purposes share this one route:
//  - the reader (src/app/books/[id]/read/page.js) points an <iframe>/epub.js
//    at ?mode=inline so the browser's native viewer renders the file instead
//    of downloading it — the Go API always sends Content-Disposition:
//    attachment, so that header is deliberately overridden here.
//  - the "Download" link on the book detail page uses ?mode=attachment
//    (the default), which just passes the Go API's own header through.
import { getAccessToken } from "@/lib/session";
import { refreshSession } from "@/lib/api";

const API_INTERNAL_URL = process.env.API_INTERNAL_URL || "http://localhost:8080";

async function fetchFile(id, accessToken) {
  return fetch(`${API_INTERNAL_URL}/api/v1/books/${id}/download`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    cache: "no-store",
  });
}

export async function GET(request, { params }) {
  const { id } = await params;
  const accessToken = await getAccessToken();
  if (!accessToken) {
    return new Response("Not authenticated", { status: 401 });
  }

  let res = await fetchFile(id, accessToken);
  if (res.status === 401 && (await refreshSession())) {
    res = await fetchFile(id, await getAccessToken());
  }

  if (!res.ok || !res.body) {
    return new Response("File not found", { status: res.status || 404 });
  }

  const mode = request.nextUrl.searchParams.get("mode") === "inline" ? "inline" : "attachment";
  const contentType = res.headers.get("content-type") || "application/octet-stream";
  const filenameMatch = (res.headers.get("content-disposition") || "").match(/filename="([^"]+)"/);
  const filename = filenameMatch ? filenameMatch[1] : `book-${id}`;

  return new Response(res.body, {
    status: 200,
    headers: {
      "Content-Type": contentType,
      "Content-Disposition": `${mode}; filename="${filename}"`,
    },
  });
}
