import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Answer,
  Assessment,
  Connect,
  ExamAnswer,
  Question,
  SubmitAnswerResponse,
} from "@/api/types";
import { fetchMyAnswer, myAnswerQueryKey } from "./useMyAnswer";

// SubmitAnswerInput describes one submission of the current question's
// selection. checkOnly=true grades without persisting (the practice-exam
// "Check" button); checkOnly=false also saves the merged answer as the
// session's latest submission (the "Next" button of both exam categories, and
// the only mode certification exams use).
export type SubmitAnswerInput = {
  question: Question;
  // selectedOptionIds carries the answer of a choice question; connections
  // carries the placed candidate→drop pairs of a drag-and-drop question. Each
  // is ignored for the other kind.
  selectedOptionIds: string[];
  connections: Connect[];
  checkOnly: boolean;
};

// SubmitAnswerResult carries the grading assessment back to the page along
// with the merged exam answer, so a persisted submission can update the cached
// my_answer without a refetch.
type SubmitAnswerResult = {
  assessment: Assessment | null;
  checkOnly: boolean;
  merged: ExamAnswer;
};

// postAnswer calls
// POST /api/examsessions/{exam_session_id}/answer[?check_only=true] with the
// merged exam answer as the body, resolving to the grading assessment. With
// check_only=true the answer is graded but not persisted.
async function postAnswer(
  examSessionId: string,
  examAnswer: ExamAnswer,
  checkOnly: boolean,
): Promise<Assessment | null> {
  const params = new URLSearchParams();
  if (checkOnly) params.set("check_only", "true");
  const res = await fetch(
    `/api/examsessions/${encodeURIComponent(examSessionId)}/answer?${params}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(examAnswer),
    },
  );
  if (!res.ok) throw new Error(`failed to submit the answer: ${res.status}`);
  const body = (await res.json()) as SubmitAnswerResponse;
  return body.assessment;
}

// useSubmitAnswer submits the current question's selection for grading. The
// exam answer is exam-scoped — the server replaces the whole submission on
// every persisted POST — so the current question's answer is merged into the
// last known submission (fetched on the spot when the cache is cold) before
// posting, preserving answers to earlier questions. Persisted submissions then
// update the cached my_answer, keeping later merges and the practice-exam
// selection restore coherent without a refetch.
export function useSubmitAnswer(examSessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      question,
      selectedOptionIds,
      connections,
      checkOnly,
    }: SubmitAnswerInput): Promise<SubmitAnswerResult> => {
      const existing = await queryClient.ensureQueryData({
        queryKey: myAnswerQueryKey(examSessionId),
        queryFn: () => fetchMyAnswer(examSessionId),
      });
      const answer: Answer = {
        questionId: question.id,
        questionType: question.type,
        options: question.options?.filter((o) =>
          selectedOptionIds.includes(o.id),
        ),
        connections:
          question.type === "drag-and-drop" ? connections : undefined,
      };
      // Replace any earlier answer to this question; keep every other one.
      const others = (existing?.answers ?? []).filter(
        (a) => a.questionId !== question.id,
      );
      const merged: ExamAnswer = { answers: [...others, answer] };
      const assessment = await postAnswer(examSessionId, merged, checkOnly);
      return { assessment, checkOnly, merged };
    },
    onSuccess: ({ checkOnly, merged }) => {
      // check_only submissions are graded but not persisted server-side, so
      // only real submissions advance the cached my_answer.
      if (!checkOnly) {
        queryClient.setQueryData(myAnswerQueryKey(examSessionId), merged);
      }
    },
  });
}
