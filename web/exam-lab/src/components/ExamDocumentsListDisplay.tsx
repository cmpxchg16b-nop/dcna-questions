"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Box, List, Typography } from "@mui/material";
import { useExamDocs } from "@/hooks/useExamDocs";
import { useCreateExamSession } from "@/hooks/useCreateExamSession";
import ExamCard from "@/components/ExamCard";
import ExamOptionsDialog from "@/components/ExamOptionsDialog";
import {
  CLIENT_SUPPORTED_QUESTION_TYPES,
  ExamExcerpt,
  ExamOptionRandomOptions,
  ExamOptionRandomQuestions,
} from "@/api/types";

type ExamDocumentsListDisplayProps = {
  generation: number;
};

// The Exams section: every exam document the caller can take, including those
// coming from their associated uploads — which is why this list subscribes to
// the shared generation: associating or un-associating an upload in the
// Uploads section changes what the server serves here. Starting a session
// refreshes the sessions section via useCreateExamSession's own invalidation,
// so no generation bump is needed for that.
export default function ExamDocumentsListDisplay({
  generation,
}: ExamDocumentsListDisplayProps) {
  const router = useRouter();
  const { data: exams, isPending } = useExamDocs(generation);
  const createSession = useCreateExamSession();
  const [examToTake, setExamToTake] = useState<ExamExcerpt | null>(null);

  // Certification exams are proctored: no customization dialog — the session
  // is created with fixed options (randomized question/option order, not
  // seekable, only client-renderable question types) and the user goes
  // straight to the exam session page. Practice exams still open the options
  // dialog so the user can customize the session.
  const handleTake = (exam: ExamExcerpt) => {
    if (exam.ExamCategory !== "certification-exam") {
      setExamToTake(exam);
      return;
    }
    createSession.mutate(
      {
        examId: exam.Id,
        options: ExamOptionRandomQuestions | ExamOptionRandomOptions,
        acceptQuestionTypes: CLIENT_SUPPORTED_QUESTION_TYPES,
      },
      {
        onSuccess: (examSessionId) => {
          const params = new URLSearchParams({
            exam_session_id: examSessionId,
          });
          router.push(`/examsession?${params}`);
        },
      },
    );
  };

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h4" component="h2" gutterBottom>
        Exams
      </Typography>
      <Typography gutterBottom>
        {!isPending && exams.length === 0
          ? "No exam is found"
          : "Here are the exams you can take"}
      </Typography>
      {isPending ? (
        <Typography>…</Typography>
      ) : (
        exams.length > 0 && (
          <List>
            {exams.map((exam) => (
              <ExamCard
                key={exam.Id}
                exam={exam}
                onTake={handleTake}
                loading={
                  createSession.isPending &&
                  createSession.variables?.examId === exam.Id
                }
              />
            ))}
          </List>
        )
      )}

      <ExamOptionsDialog
        exam={examToTake}
        onClose={() => setExamToTake(null)}
      />
    </Box>
  );
}
