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
  ExamExcerpt,
  ExamOptionRandomOptions,
  ExamOptionRandomQuestions,
  ExamOptionSeekable,
} from "@/api/types";

type ExamOptionsDialogProps = {
  exam: ExamExcerpt | null;
  onClose: () => void;
};

// Confirmation dialog shown before starting an exam: displays the exam's
// metadata and lets the user pick ExamOptions (randomized question/option
// order) for the new session. Renders nothing visible when exam is null.
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

  const options =
    (randomQuestions ? ExamOptionRandomQuestions : 0) |
    (randomOptions ? ExamOptionRandomOptions : 0) |
    (seekable ? ExamOptionSeekable : 0);

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
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          loading={createSession.isPending}
          onClick={() =>
            createSession.mutate(
              { examId: exam.Id, options },
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
