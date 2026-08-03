import { useQuery } from "@tanstack/react-query";
import { ExamReports, ExamTrackingListResponse } from "@/api/types";

// fetchExamTrackings calls GET /api/examtrackings, which returns the caller's
// finished-exam reports as a single JSON object {"exam_reports": [...]}. The
// session middleware identifies the caller via the session_id cookie, which
// same-origin fetch sends automatically.
async function fetchExamTrackings(): Promise<ExamReports> {
  const res = await fetch("/api/examtrackings");
  if (!res.ok)
    throw new Error(`failed to fetch exam trackings: ${res.status}`);
  const body = (await res.json()) as ExamTrackingListResponse;
  return body.exam_reports;
}

// useExamTrackings fetches and caches the caller's exam reports under the
// "examtrackings" query key. `data` is always a defined array (empty while the
// first request is pending), and `isPending` is true during the initial fetch
// so callers can show a loading placeholder.
export function useExamTrackings(): { data: ExamReports; isPending: boolean } {
  const { data = [], isPending } = useQuery({
    queryKey: ["examtrackings"],
    queryFn: fetchExamTrackings,
  });
  return { data, isPending };
}
