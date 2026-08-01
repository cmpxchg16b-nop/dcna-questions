import { useMutation, useQueryClient } from "@tanstack/react-query";

// endExamSession calls DELETE /api/examsessions/{exam_session_id}, which
// terminates the session and responds 204 No Content (404 when the session is
// already gone). The caller's session_id cookie is sent automatically
// (same-origin fetch).
async function endExamSession(examSessionId: string): Promise<void> {
  const res = await fetch(
    `/api/examsessions/${encodeURIComponent(examSessionId)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw new Error(`failed to end exam session: ${res.status}`);
}

// useEndExamSession terminates an exam session by id. On success it
// invalidates the "examsessions" query (the key used by useExamSessions) so
// the sessions list refetches without the ended session.
export function useEndExamSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: endExamSession,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["examsessions"] });
    },
  });
}
