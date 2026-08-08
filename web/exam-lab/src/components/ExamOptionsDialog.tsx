"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  Box,
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
} from "@mui/material";
import { useCreateExamSession } from "@/hooks/useCreateExamSession";
import { useProfile } from "@/hooks/useProfile";
import {
  CLIENT_SUPPORTED_QUESTION_TYPES,
  ExamExcerpt,
  ExamOptionRandomOptions,
  ExamOptionRandomQuestions,
  ExamOptionSeekable,
  ExamOptionSendExamReportEmail,
  QuestionType,
  VISITOR_SUBJECT_PREFIX,
} from "@/api/types";
import { useTranslation } from "react-i18next";

type ExamOptionsDialogProps = {
  exam: ExamExcerpt | null;
  onClose: () => void;
};

// Confirmation dialog shown before starting an exam: displays the exam's
// metadata and lets the user pick ExamOptions (randomized question/option
// order, seekability) and restrict the question types for the new session.
// Renders nothing visible when exam is null.
export default function ExamOptionsDialog({
  exam,
  onClose,
}: ExamOptionsDialogProps) {
  return (
    // fullWidth keeps the dialog at the sm breakpoint width instead of
    // shrinking to fit the (possibly short, e.g. CJK) checkbox labels.
    <Dialog open={exam !== null} onClose={onClose} fullWidth maxWidth="sm">
      {/* The form holds the checkbox state; MUI unmounts the Dialog's children
        when it closes, so both checkboxes reset to unchecked on every open. */}
      {exam && <ExamOptionsForm exam={exam} onClose={onClose} />}
    </Dialog>
  );
}

function ExamOptionsForm({
  exam,
  onClose,
}: {
  exam: ExamExcerpt;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const router = useRouter();
  const createSession = useCreateExamSession();
  const { data: profile } = useProfile();
  const [randomQuestions, setRandomQuestions] = useState(false);
  const [randomOptions, setRandomOptions] = useState(false);
  const [seekable, setSeekable] = useState(false);
  const [sendExamReportEmail, setSendExamReportEmail] = useState(false);
  // A visitor has no email address to deliver the report to: hide the
  // mailing consent checkbox entirely (its state stays false, so the option
  // bit is never set — the server masks it off for visitors anyway).
  const isVisitor =
    profile?.subject_id.startsWith(VISITOR_SUBJECT_PREFIX) ?? false;
  // Question types absent from CLIENT_SUPPORTED_QUESTION_TYPES cannot be
  // rendered by the client: they start unchecked and are disabled below.
  const supported = (qt: QuestionType) =>
    CLIENT_SUPPORTED_QUESTION_TYPES.includes(qt);
  const [singleChoice, setSingleChoice] = useState(supported("single-choice"));
  const [multipleChoice, setMultipleChoice] = useState(
    supported("multiple-choice"),
  );
  const [dragAndDrop, setDragAndDrop] = useState(supported("drag-and-drop"));

  const options =
    (randomQuestions ? ExamOptionRandomQuestions : 0) |
    (randomOptions ? ExamOptionRandomOptions : 0) |
    (seekable ? ExamOptionSeekable : 0) |
    (sendExamReportEmail ? ExamOptionSendExamReportEmail : 0);

  // The accepted question types, in the order the checkboxes appear. An empty
  // array would mean "accept every type" to the server, so the Take button is
  // disabled instead when nothing is selected.
  const acceptQuestionTypes: QuestionType[] = [
    ...(singleChoice ? ["single-choice" as const] : []),
    ...(multipleChoice ? ["multiple-choice" as const] : []),
    ...(dragAndDrop ? ["drag-and-drop" as const] : []),
  ];

  return (
    <>
      <DialogTitle>{exam.Title}</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {exam.ShortName} · {exam.Code}
        </DialogContentText>
        <Box sx={{ display: "flex", flexDirection: "column" }}>
          <FormControlLabel
            control={
              <Checkbox
                checked={randomQuestions}
                onChange={(e) => setRandomQuestions(e.target.checked)}
              />
            }
            label={t("examOptions.randomQuestions")}
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={randomOptions}
                onChange={(e) => setRandomOptions(e.target.checked)}
              />
            }
            label={t("examOptions.randomOptions")}
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={seekable}
                onChange={(e) => setSeekable(e.target.checked)}
              />
            }
            label={t("examOptions.seekable")}
          />
          {!isVisitor && (
            <FormControlLabel
              control={
                <Checkbox
                  checked={sendExamReportEmail}
                  onChange={(e) => setSendExamReportEmail(e.target.checked)}
                />
              }
              label={t("examOptions.sendExamReportEmail")}
            />
          )}
        </Box>
        <DialogContentText sx={{ mt: 2 }}>
          {t("examOptions.questionTypes")}
        </DialogContentText>
        <Box sx={{ display: "flex", flexDirection: "column" }}>
          <FormControlLabel
            control={
              <Checkbox
                checked={singleChoice}
                disabled={!supported("single-choice")}
                onChange={(e) => setSingleChoice(e.target.checked)}
              />
            }
            label={t("examOptions.singleChoice")}
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={multipleChoice}
                disabled={!supported("multiple-choice")}
                onChange={(e) => setMultipleChoice(e.target.checked)}
              />
            }
            label={t("examOptions.multipleChoice")}
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={dragAndDrop}
                disabled={!supported("drag-and-drop")}
                onChange={(e) => setDragAndDrop(e.target.checked)}
              />
            }
            label={t("examOptions.dragAndDrop")}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t("common.cancel")}</Button>
        <Button
          variant="contained"
          loading={createSession.isPending}
          disabled={acceptQuestionTypes.length === 0}
          onClick={() =>
            createSession.mutate(
              { examId: exam.Id, options, acceptQuestionTypes },
              {
                // Session created: go straight to the exam session page, same
                // as the certification-exam flow in the home page.
                onSuccess: (examSessionId) => {
                  const params = new URLSearchParams({
                    exam_session_id: examSessionId,
                  });
                  router.push(`/examsession?${params}`);
                },
              },
            )
          }
        >
          {t("exam.take")}
        </Button>
      </DialogActions>
    </>
  );
}
