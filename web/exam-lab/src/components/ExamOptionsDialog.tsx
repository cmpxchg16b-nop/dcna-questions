"use client";

import { useState } from "react";
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
import {
  CLIENT_SUPPORTED_QUESTION_TYPES,
  ExamExcerpt,
  ExamOptionRandomOptions,
  ExamOptionRandomQuestions,
  ExamOptionSeekable,
  QuestionType,
} from "@/api/types";

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
    <Dialog open={exam !== null} onClose={onClose}>
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
  const createSession = useCreateExamSession();
  const [randomQuestions, setRandomQuestions] = useState(false);
  const [randomOptions, setRandomOptions] = useState(false);
  const [seekable, setSeekable] = useState(false);
  // Question types absent from CLIENT_SUPPORTED_QUESTION_TYPES cannot be
  // rendered by the client: they start unchecked and are disabled below.
  const supported = (t: QuestionType) =>
    CLIENT_SUPPORTED_QUESTION_TYPES.includes(t);
  const [singleChoice, setSingleChoice] = useState(supported("single-choice"));
  const [multipleChoice, setMultipleChoice] = useState(
    supported("multiple-choice"),
  );
  const [dragAndDrop, setDragAndDrop] = useState(supported("drag-and-drop"));

  const options =
    (randomQuestions ? ExamOptionRandomQuestions : 0) |
    (randomOptions ? ExamOptionRandomOptions : 0) |
    (seekable ? ExamOptionSeekable : 0);

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
            label="Randomized questions order"
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={randomOptions}
                onChange={(e) => setRandomOptions(e.target.checked)}
              />
            }
            label="Randomized options order"
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={seekable}
                onChange={(e) => setSeekable(e.target.checked)}
              />
            }
            label="Seekable (allow going back to previous questions)"
          />
        </Box>
        <DialogContentText sx={{ mt: 2 }}>Question types</DialogContentText>
        <Box sx={{ display: "flex", flexDirection: "column" }}>
          <FormControlLabel
            control={
              <Checkbox
                checked={singleChoice}
                disabled={!supported("single-choice")}
                onChange={(e) => setSingleChoice(e.target.checked)}
              />
            }
            label="Single-choice"
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={multipleChoice}
                disabled={!supported("multiple-choice")}
                onChange={(e) => setMultipleChoice(e.target.checked)}
              />
            }
            label="Multiple-choice"
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={dragAndDrop}
                disabled={!supported("drag-and-drop")}
                onChange={(e) => setDragAndDrop(e.target.checked)}
              />
            }
            label="Drag-and-drop"
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          loading={createSession.isPending}
          disabled={acceptQuestionTypes.length === 0}
          onClick={() =>
            createSession.mutate(
              { examId: exam.Id, options, acceptQuestionTypes },
              {
                onSuccess: onClose,
              },
            )
          }
        >
          Take
        </Button>
      </DialogActions>
    </>
  );
}
