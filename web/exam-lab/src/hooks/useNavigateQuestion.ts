import { useRef } from "react";
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

// NavigateInput describes one navigation step to the question at index.
//
// Backward moves (seek: true, the "Previous" button) always reposition the
// cursor to index via SeekCursorTo first, then read through the repositioned
// cursor.
//
// Forward moves (seek: false, the "Start"/"Next" buttons) read through the
// tracked cursor. GetNextQuestion is never called on its own — only in
// response to these button moves.
export type NavigateInput = {
  index: number;
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

// useNavigateQuestion drives question navigation within an exam session. The
// mutation's `data` is the current QuestionPosition, so the page derives the
// on-screen question and index from it without extra state or effects. On
// success it also syncs current_question_index and current_question into the
// cached session summary so anything reading ["examsession", examSessionId]
// stays coherent. seekable (from the session's options bitmask) selects the
// cursor-restoration strategy below.
export function useNavigateQuestion(examSessionId: string, seekable: boolean) {
  const queryClient = useQueryClient();
  // cursorRef tracks the opaque cursor most recently returned by the server
  // (by a seek or a question read). It is null until this page view has served
  // a question — e.g. right after a page reload. A null cursor reads index 0,
  // so it is exactly what "Start" relies on; a forward move to any other index
  // with a null cursorRef must first reposition the cursor: by a seek when the
  // session is seekable, or by replaying GetNextQuestion from an empty cursor
  // when it is not.
  const cursorRef = useRef<string | null>(null);
  return useMutation({
    mutationFn: async ({
      index,
      seek,
    }: NavigateInput): Promise<QuestionPosition> => {
      let cursor = cursorRef.current;
      if (seek) {
        // Backward move: reposition the cursor to index first (requires a
        // seekable session; the page disables Previous otherwise), then read
        // through the repositioned cursor.
        cursor = await seekCursor(examSessionId, cursor, index);
      } else if (cursor === null && index !== 0) {
        if (seekable) {
          // The tracked cursor is lost (e.g. the page was reloaded): mint a
          // fresh one already positioned at the target index.
          cursor = await seekCursor(examSessionId, null, index);
        } else {
          // Non-seekable sessions cannot seek, so the cursor is restored by
          // replaying GetNextQuestion from an empty cursor up to the target
          // index. Every replayed read advances the server-side current
          // question index, so the target is captured up front in `index`
          // (the loop never consults the session) and only the final read's
          // question and cursor are kept.
          let question: Question | null = null;
          for (let i = 0; i <= index; i++) {
            const res = await fetchNextQuestion(examSessionId, cursor);
            if (!res.question) {
              throw new Error(`the server served no question at index ${i}`);
            }
            cursor = res.cursor_id;
            question = res.question;
          }
          if (!question) {
            throw new Error(`the server served no question at index ${index}`);
          }
          cursorRef.current = cursor;
          return { index, question, nextCursor: cursor };
        }
      }
      const { question, cursor_id } = await fetchNextQuestion(
        examSessionId,
        cursor,
      );
      if (!question) {
        throw new Error(`the server served no question at index ${index}`);
      }
      cursorRef.current = cursor_id;
      return { index, question, nextCursor: cursor_id };
    },
    onSuccess: (position) => {
      queryClient.setQueryData<ExamSessionSummary>(
        ["examsession", examSessionId],
        (old) =>
          old
            ? {
                ...old,
                current_question_index: position.index,
                current_question: position.question,
              }
            : old,
      );
    },
  });
}
