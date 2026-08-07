import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { UserUploadListResponse, UserUploads } from "@/api/types";

// fetchUploads calls GET /api/useruploads, which returns the caller's uploads
// as a single JSON object {"uploads": [...]}. The session middleware
// identifies the caller via the session_id cookie, which same-origin fetch
// sends automatically.
async function fetchUploads(): Promise<UserUploads> {
  const res = await fetch("/api/useruploads");
  if (!res.ok) throw new Error(`failed to fetch uploads: ${res.status}`);
  const body = (await res.json()) as UserUploadListResponse;
  return body.uploads;
}

// useUploads fetches and caches the caller's uploads under the "useruploads"
// query key. `generation` is appended to the key, so bumping it mounts a fresh
// query and refetches the list — the mechanism by which the parent page lets
// one section refresh another (prefix-based invalidations such as
// invalidateQueries({queryKey: ["useruploads"]}) still match). `data` is
// still match). `data` is always a defined array (empty while the first
// request is pending). `isPending` is true only for the very first fetch: on
// generation bumps keepPreviousData serves the previous list while the new
// query fetches, so callers keep rendering the list instead of collapsing to
// the loading placeholder (which would shrink the page and yank the scroll
// position).
export function useUploads(generation: number): {
  data: UserUploads;
  isPending: boolean;
} {
  const { data = [], isPending } = useQuery({
    queryKey: ["useruploads", generation],
    queryFn: fetchUploads,
    placeholderData: keepPreviousData,
  });
  return { data, isPending };
}
