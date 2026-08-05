import { useQuery } from "@tanstack/react-query";
import { ExamSessionListResponse, ExamSessions } from "@/api/types";

// fetchExamSessions calls GET /api/examsessions, which returns the caller's
// active sessions as a single JSON object {"exam_sessions": [...]}. The
// session middleware identifies the caller via the session_id cookie, which
// same-origin fetch sends automatically.
async function fetchExamSessions(): Promise<ExamSessions> {
  const res = await fetch("/api/examsessions");
  if (!res.ok) throw new Error(`failed to fetch exam sessions: ${res.status}`);
  const body = (await res.json()) as ExamSessionListResponse;
  return body.exam_sessions;
}

// useExamSessions fetches and caches the caller's exam sessions under the
// "examsessions" query key. `generation` is appended to the key, so bumping it
// mounts a fresh query and refetches the list — the mechanism by which the
// parent page lets one section refresh another (prefix-based invalidations
// such as invalidateQueries({queryKey: ["examsessions"]}) still match). `data`
// is always a defined array (empty while the first request is pending), and
// `isPending` is true during the initial fetch so callers can show a loading
// placeholder.
export function useExamSessions(generation: number): {
  data: ExamSessions;
  isPending: boolean;
} {
  const { data = [], isPending } = useQuery({
    queryKey: ["examsessions", generation],
    queryFn: fetchExamSessions,
  });
  return { data, isPending };
}
