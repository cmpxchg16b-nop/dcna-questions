import { useQuery } from "@tanstack/react-query";
import { ExamAnswer, MyAnswerResponse } from "@/api/types";

// myAnswerQueryKey is the cache key for a session's saved exam answer. It is
// exported so useSubmitAnswer can read and update the same cache entry.
export function myAnswerQueryKey(examSessionId: string) {
  return ["myanswer", examSessionId] as const;
}

// fetchMyAnswer calls GET /api/examsessions/{exam_session_id}/my_answer, which
// returns the caller's last saved submission as {"exam_answer": {...} or null}
// (null when no answer has been submitted yet). The answer is exam-scoped: a
// single document holding one answer per answered question. The caller's
// session_id cookie is sent automatically (same-origin fetch).
export async function fetchMyAnswer(
  examSessionId: string,
): Promise<ExamAnswer | null> {
  const res = await fetch(
    `/api/examsessions/${encodeURIComponent(examSessionId)}/my_answer`,
  );
  if (!res.ok) throw new Error(`failed to fetch my answer: ${res.status}`);
  const body = (await res.json()) as MyAnswerResponse;
  return body.exam_answer;
}

// useMyAnswer fetches and caches the session's saved exam answer under
// ["myanswer", examSessionId]. It is enabled once the session has started (a
// question is on screen): the practice-exam flow needs it to restore
// previously submitted selections — its footer shows the "Skip (loading)"
// state until this first fetch resolves — and both exam categories merge new
// submissions into it (see useSubmitAnswer). The cache is kept current by
// useSubmitAnswer after each persisted submission, so navigating between
// questions does not refetch.
export function useMyAnswer(
  examSessionId: string | null | undefined,
  enabled: boolean,
) {
  return useQuery({
    queryKey: ["myanswer", examSessionId],
    queryFn: () => fetchMyAnswer(examSessionId as string),
    enabled: !!examSessionId && enabled,
  });
}
