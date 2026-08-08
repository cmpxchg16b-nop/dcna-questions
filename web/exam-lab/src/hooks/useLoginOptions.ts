import { useQuery } from "@tanstack/react-query";
import { LoginOptions } from "@/api/types";

// fetchLoginOptions calls GET /api/login/loginoptions. The endpoint sits on
// the JWT whitelist (it feeds the login page itself), so it answers for
// logged-out callers too.
async function fetchLoginOptions(): Promise<LoginOptions> {
  const res = await fetch("/api/login/loginoptions");
  if (!res.ok) throw new Error(`failed to fetch login options: ${res.status}`);
  return (await res.json()) as LoginOptions;
}

// useLoginOptions fetches and caches the login page's IdP list under the
// "loginOptions" query key. The list only changes with the server
// configuration, so it is never considered stale.
export function useLoginOptions() {
  return useQuery({
    queryKey: ["loginOptions"],
    queryFn: fetchLoginOptions,
    staleTime: Infinity,
  });
}
