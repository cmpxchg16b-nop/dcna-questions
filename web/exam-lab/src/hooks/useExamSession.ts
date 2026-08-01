import { useQuery } from "@tanstack/react-query";
import { ExamSessionResponse, ExamSessionSummary } from "@/api/types";

// fetchExamSession calls GET /api/examsessions/{exam_session_id}, which returns
// the caller's session as {"exam_session": {...}}, including its current
// question index. The caller's session_id cookie is sent automatically
// (same-origin fetch).
async function fetchExamSession(
  examSessionId: string,
): Promise<ExamSessionSummary> {
  const res = await fetch(
    `/api/examsessions/${encodeURIComponent(examSessionId)}`,
  );
  if (!res.ok) throw new Error(`failed to fetch exam session: ${res.status}`);
  const body = (await res.json()) as ExamSessionResponse;
  return body.exam_session;
}

// useExamSession fetches and caches a single exam session under the
// ["examsession", examSessionId] query key. It is disabled until an
// examSessionId is provided, so the exam session page can call it unconditionally
// and read the id from the URL.
export function useExamSession(examSessionId: string | null | undefined) {
  return useQuery({
    queryKey: ["examsession", examSessionId],
    queryFn: () => fetchExamSession(examSessionId as string),
    enabled: !!examSessionId,
  });
}
