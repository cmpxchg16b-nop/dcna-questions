import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ExamReports, ExamTrackingListResponse } from "@/api/types";

// fetchExamTrackings calls GET /api/examtrackings, which returns the caller's
// finished-exam reports as a single JSON object {"exam_reports": [...]}. The
// session middleware identifies the caller via the session_id cookie, which
// same-origin fetch sends automatically.
async function fetchExamTrackings(): Promise<ExamReports> {
  const res = await fetch("/api/examtrackings");
  if (!res.ok) throw new Error(`failed to fetch exam trackings: ${res.status}`);
  const body = (await res.json()) as ExamTrackingListResponse;
  return body.exam_reports;
}

// useExamTrackings fetches and caches the caller's exam reports under the
// "examtrackings" query key. `generation` is appended to the key, so bumping
// it mounts a fresh query and refetches the list — the mechanism by which the
// parent page lets one section refresh another (prefix-based invalidations
// such as invalidateQueries({queryKey: ["examtrackings"]}) still match).
// `data` is always a defined array (empty while the first request is
// pending). `isPending` is true only for the very first fetch: on generation
// bumps keepPreviousData serves the previous list while the new query
// fetches, so callers keep rendering the list instead of collapsing to the
// loading placeholder (which would shrink the page and yank the scroll
// position).
export function useExamTrackings(generation: number): {
  data: ExamReports;
  isPending: boolean;
} {
  const { data = [], isPending } = useQuery({
    queryKey: ["examtrackings", generation],
    queryFn: fetchExamTrackings,
    placeholderData: keepPreviousData,
  });
  return { data, isPending };
}
