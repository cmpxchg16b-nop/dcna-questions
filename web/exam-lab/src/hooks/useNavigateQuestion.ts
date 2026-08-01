import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ExamSessionSummary,
  NextQuestionResponse,
  Question,
  SeekCursorResponse,
} from "@/api/types";

// QuestionPosition is the client's view of where the session's read cursor
// sits: the question on screen, its virtual index, and the opaque cursor token
// to continue forward with (null once the last question has been served).
export type QuestionPosition = {
  index: number;
  question: Question;
  nextCursor: string | null;
};

// NavigateInput describes one navigation step.
//
// Forward moves (seek: false) read the question at index through the current
// cursor; a null cursor is treated by GetNextQuestion as "from the beginning",
// which is what starting an exam relies on.
//
// Backward and resume moves (seek: true) first reposition the cursor to index
// via SeekCursorTo — which requires a seekable session and invalidates the
// passed cursor, if any — then read through the repositioned cursor.
export type NavigateInput = {
  index: number;
  cursor: string | null;
  seek: boolean;
};

// fetchNextQuestion calls
// GET /api/examsessions/{exam_session_id}/questions?cursor_id=<cursor>,
// returning the served question (null when none remain) and the cursor to
// continue from (null once the last question was served). The caller's
// session_id cookie is sent automatically (same-origin fetch).
async function fetchNextQuestion(
  examSessionId: string,
  cursor: string | null,
): Promise<NextQuestionResponse> {
  const params = new URLSearchParams();
  if (cursor) params.set("cursor_id", cursor);
  const res = await fetch(
    `/api/examsessions/${encodeURIComponent(examSessionId)}/questions?${params}`,
  );
  if (!res.ok)
    throw new Error(`failed to fetch the next question: ${res.status}`);
  return (await res.json()) as NextQuestionResponse;
}

// seekCursor calls
// PUT /api/examsessions/{exam_session_id}/cursors?index=<n>&cursor_id=<cursor>
// and resolves to the repositioned cursor, which must be used for the next
// read (the passed cursor, when present, is invalidated by the seek). It fails
// when the session is not seekable or the index is out of range.
async function seekCursor(
  examSessionId: string,
  cursor: string | null,
  index: number,
): Promise<string> {
  const params = new URLSearchParams({ index: String(index) });
  if (cursor) params.set("cursor_id", cursor);
  const res = await fetch(
    `/api/examsessions/${encodeURIComponent(examSessionId)}/cursors?${params}`,
    { method: "PUT" },
  );
  if (!res.ok)
    throw new Error(`failed to seek to question ${index + 1}: ${res.status}`);
  const body = (await res.json()) as SeekCursorResponse;
  return body.cursor_id;
}

// navigateQuestion performs one navigation step: an optional seek followed by
// reading the question at the target index. It resolves to the new position,
// including the cursor to continue forward from.
async function navigateQuestion(
  examSessionId: string,
  { index, cursor, seek }: NavigateInput,
): Promise<QuestionPosition> {
  const readCursor = seek
    ? await seekCursor(examSessionId, cursor, index)
    : cursor;
  const { question, cursor_id } = await fetchNextQuestion(
    examSessionId,
    readCursor,
  );
  if (!question) {
    throw new Error(`the server served no question at index ${index}`);
  }
  return { index, question, nextCursor: cursor_id };
}

// useNavigateQuestion drives question navigation within an exam session. The
// mutation's `data` is the current QuestionPosition, so the page derives the
// on-screen question and index from it without extra state or effects. On
// success it also syncs current_question_index into the cached session summary
// so anything reading ["examsession", examSessionId] stays coherent.
export function useNavigateQuestion(examSessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: NavigateInput) =>
      navigateQuestion(examSessionId, input),
    onSuccess: (position) => {
      queryClient.setQueryData<ExamSessionSummary>(
        ["examsession", examSessionId],
        (old) =>
          old ? { ...old, current_question_index: position.index } : old,
      );
    },
  });
}
