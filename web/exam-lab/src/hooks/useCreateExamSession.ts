import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  CreateExamSessionRequest,
  CreateExamSessionResponse,
  QuestionType,
} from "@/api/types";

// CreateExamSessionInput is what the mutation consumes: the exam document id,
// an ExamOptions bitmask (see the ExamOption* constants in @/api/types), and
// the question types the session should serve (an empty array accepts every
// type).
export type CreateExamSessionInput = {
  examId: string;
  options: number;
  acceptQuestionTypes: QuestionType[];
};

// createExamSession calls POST /api/examsessions with
// {"exam_id": "...", "options": <bitmask>, "accept_question_types": [...]} and
// resolves to the new session id. The caller's session_id cookie is sent
// automatically (same-origin fetch).
async function createExamSession({
  examId,
  options,
  acceptQuestionTypes,
}: CreateExamSessionInput): Promise<string> {
  const payload: CreateExamSessionRequest = {
    exam_id: examId,
    options,
    accept_question_types: acceptQuestionTypes,
  };
  const res = await fetch("/api/examsessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`failed to create exam session: ${res.status}`);
  const body = (await res.json()) as CreateExamSessionResponse;
  return body.exam_session_id;
}

// useCreateExamSession starts a new exam session for the given exam document
// id. On success it invalidates the "examsessions" query (the key used by
// useExamSessions) so the sessions list refetches with the new session.
export function useCreateExamSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createExamSession,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["examsessions"] });
    },
  });
}
