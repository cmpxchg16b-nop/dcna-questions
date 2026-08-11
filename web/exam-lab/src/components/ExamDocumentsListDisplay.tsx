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
  LabelFilter,
} from "@/api/types";
import { useTranslation } from "react-i18next";

type ExamDocumentsListDisplayProps = {
  generation: number;
  // labelFilter narrows the list server-side via /api/examdocs/bylabel: an
  // exam is listed only when every key matches at least one of its accepted
  // values (OR within a key, AND across keys). The parent page derives it
  // from the URL's query parameters.
  labelFilter?: LabelFilter;
};

// The Exams section: every exam document the caller can take, including those
// coming from their associated uploads — which is why this list subscribes to
// the shared generation: associating or un-associating an upload in the
// Uploads section changes what the server serves here. Starting a session
// refreshes the sessions section via useCreateExamSession's own invalidation,
// so no generation bump is needed for that.
export default function ExamDocumentsListDisplay({
  generation,
  labelFilter,
}: ExamDocumentsListDisplayProps) {
  const { t } = useTranslation();
  const router = useRouter();
  const { data: exams, isPending } = useExamDocs(generation, labelFilter);
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
        {t("exams.title")}
      </Typography>
      <Typography gutterBottom>
        {!isPending && exams.length === 0
          ? t("exams.empty")
          : t("exams.nonEmpty")}
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
