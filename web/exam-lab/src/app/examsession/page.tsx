"use client";

import { Fragment, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Tooltip,
  Typography,
} from "@mui/material";
import { useExamSession } from "@/hooks/useExamSession";
import { useEndExamSession } from "@/hooks/useEndExamSession";
import { useNavigateQuestion } from "@/hooks/useNavigateQuestion";
import QuestionCard from "@/components/QuestionCard";
import { ExamOptionSeekable } from "@/api/types";

export default function Page() {
  const router = useRouter();
  const searchParams = useSearchParams();

  // The page is keyed off the session id alone; the session's metadata (title,
  // shortname, code, total question count) and current question index are all
  // sourced from the server rather than query params.
  const examSessionId = searchParams.get("exam_session_id");

  const { data: session, isPending, isError } = useExamSession(examSessionId);
  const endSession = useEndExamSession();
  // seekable comes from the session's options bitmask and selects how the
  // navigation hook repositions a lost cursor (seek vs. replay).
  const seekable = session
    ? (session.options & ExamOptionSeekable) !== 0
    : false;
  const navigate = useNavigateQuestion(examSessionId ?? "", seekable);

  const [confirmEndOpen, setConfirmEndOpen] = useState(false);

  // position is the question fetched by this page view's latest navigation
  // (Start/Previous/Next). It is null until this page view has navigated, in
  // which case the server-side current_question_index/current_question are the
  // fallback: the two are coherent, so a non-negative index means the session's
  // current_question is the last question it served (e.g. before a reload).
  const position = navigate.data ?? null;
  const currentQuestionIndex =
    position?.index ?? session?.current_question_index ?? -1;
  // The effective current question: the fetched one when this page view has
  // navigated, else the session's last visited one.
  const effectiveQuestion =
    position?.question ?? session?.current_question ?? null;

  const numQuestions = session?.exam_excerpt.NumQuestions ?? 0;
  const isLastQuestion = currentQuestionIndex >= numQuestions - 1;

  if (!examSessionId) {
    return (
      <Box sx={{ mt: 4 }}>
        <Typography color="error">Missing exam_session_id.</Typography>
      </Box>
    );
  }

  if (isPending) {
    return (
      <Box sx={{ mt: 4 }}>
        <Typography>…</Typography>
      </Box>
    );
  }

  if (isError || !session) {
    return (
      <Box sx={{ mt: 4 }}>
        <Typography color="error">
          Failed to load exam session {examSessionId}.
        </Typography>
      </Box>
    );
  }

  const excerpt = session.exam_excerpt;

  const goToQuestion = (nextIndex: number) => {
    if (nextIndex === currentQuestionIndex + 1) {
      // go next: read through the cursor tracked by the navigation hook.
      navigate.mutate({ index: nextIndex, seek: false });
    } else if (nextIndex === currentQuestionIndex - 1) {
      // go previous: reposition the cursor to the previous index (requires a
      // seekable session), then read the question there.
      navigate.mutate({ index: nextIndex, seek: true });
    } else {
      // unsupported
      console.error(`Unsupported index: ${nextIndex}`);
    }
  };

  const startExam = () => goToQuestion(0);

  return (
    <Box>
      <Box sx={{ mt: 4 }}>
        <Typography variant="h4" component="h2" gutterBottom>
          {excerpt.Title}
        </Typography>
        <Typography gutterBottom variant="body2" color="text.secondary">
          {excerpt.ShortName} · {excerpt.Code} · {numQuestions}{" "}
          {numQuestions === 1 ? "question" : "questions"}
        </Typography>
      </Box>

      <Box sx={{ mt: 4 }}>
        {currentQuestionIndex === -1 ? (
          <Typography>
            Welcome! After you are prepared, click the &quot;Start Exam&quot;
            Button to start exam.
          </Typography>
        ) : (
          effectiveQuestion && (
            <Fragment>
              <Typography gutterBottom>
                Question {currentQuestionIndex + 1} of {numQuestions}
              </Typography>
              {/* Keying by question id resets the card's selection state every
                  time a new question is served. */}
              <QuestionCard
                key={effectiveQuestion.id}
                question={effectiveQuestion}
              />
            </Fragment>
          )
        )}

        {navigate.isError && (
          <Typography color="error" sx={{ mt: 1 }}>
            {navigate.error.message}
          </Typography>
        )}

        {/* End Exam replaces Next in the same flex slot on the last question,
            so it occupies exactly the same position. */}
        <Box sx={{ display: "flex", justifyContent: "space-between", mt: 2 }}>
          <Tooltip title={seekable ? "" : "This exam session is not seekable"}>
            {/* MUI Tooltips don't fire on disabled buttons, hence the span. */}
            <span>
              <Button
                variant="contained"
                disabled={
                  currentQuestionIndex <= 0 || !seekable || navigate.isPending
                }
                onClick={() => goToQuestion(currentQuestionIndex - 1)}
              >
                Previous
              </Button>
            </span>
          </Tooltip>
          {currentQuestionIndex === -1 ? (
            <Button
              variant="contained"
              loading={navigate.isPending}
              onClick={startExam}
            >
              Start Exam
            </Button>
          ) : isLastQuestion ? (
            <Button
              variant="contained"
              color="error"
              onClick={() => setConfirmEndOpen(true)}
            >
              End Exam
            </Button>
          ) : (
            <Button
              variant="contained"
              loading={navigate.isPending}
              onClick={() => goToQuestion(currentQuestionIndex + 1)}
            >
              Next
            </Button>
          )}
        </Box>
      </Box>

      <Dialog open={confirmEndOpen} onClose={() => setConfirmEndOpen(false)}>
        <DialogTitle>End exam?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to end the exam? Your answers will be
            submitted for grading. This cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmEndOpen(false)}>Cancel</Button>
          <Button
            color="error"
            variant="contained"
            loading={endSession.isPending}
            onClick={() => {
              endSession.mutate(examSessionId, {
                onSuccess: () => router.push("/"),
              });
            }}
          >
            End Exam
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
