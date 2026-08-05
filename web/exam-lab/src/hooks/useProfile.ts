import { useQuery } from "@tanstack/react-query";
import { ProfileResponse } from "@/api/types";

// fetchProfile calls GET /api/profile. The endpoint sits behind the JWT
// middleware, so an unauthenticated caller gets a 401 — surfaced here as an
// error so callers can simply hide profile affordances when logged out.
async function fetchProfile(): Promise<ProfileResponse> {
  const res = await fetch("/api/profile");
  if (!res.ok) throw new Error(`failed to fetch profile: ${res.status}`);
  return (await res.json()) as ProfileResponse;
}

// useProfile fetches and caches the caller's profile under the "profile"
// query key. Retries are disabled: the expected failure is a 401 while logged
// out, which retrying cannot fix.
export function useProfile() {
  return useQuery({
    queryKey: ["profile"],
    queryFn: fetchProfile,
    retry: false,
  });
}
