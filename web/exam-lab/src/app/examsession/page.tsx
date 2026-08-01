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
  const navigate = useNavigateQuestion(examSessionId ?? "");

  const [confirmEndOpen, setConfirmEndOpen] = useState(false);

  // position is the question currently on screen plus the cursor to continue
  // from; it doubles as the cursor store the navigation API flows require. It
  // is null until this page view has served a question, in which case the
  // server-side current_question_index is the fallback: a non-negative server
  // index then means the session was interrupted (e.g. the page was reloaded)
  // and its question/cursor must be recovered through a seek.
  const position = navigate.data ?? null;
  const serverIndex = session?.current_question_index ?? -1;
  const currentQuestionIndex = position?.index ?? serverIndex;
  const interrupted = position === null && serverIndex >= 0;

  const numQuestions = session?.exam_excerpt.NumQuestions ?? 0;
  const isLastQuestion = currentQuestionIndex >= numQuestions - 1;
  const seekable = session
    ? (session.options & ExamOptionSeekable) !== 0
    : false;

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
      // go next: read through the current forward cursor. A null cursor reads
      // from the beginning of the session, which is what Start Exam relies on.
      navigate.mutate({
        index: nextIndex,
        cursor: position?.nextCursor ?? null,
        seek: false,
      });
    } else if (nextIndex === currentQuestionIndex - 1) {
      // go previous: reposition the cursor to the previous index (requires a
      // seekable session and invalidates the current cursor), then read the
      // question through the repositioned cursor the server returned.
      navigate.mutate({
        index: nextIndex,
        cursor: position?.nextCursor ?? null,
        seek: true,
      });
    } else {
      // unsupported
      console.error(`Unsupported index: ${nextIndex}`);
    }
  };

  const startExam = () => goToQuestion(0);

  // resumeExam recovers the interrupted session by seeking to the server's
  // current question index with no cursor (SeekCursorTo mints a fresh one)
  // and reading the question there. Only possible for seekable sessions.
  const resumeExam = () =>
    navigate.mutate({ index: serverIndex, cursor: null, seek: true });

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
        {interrupted ? (
          <Typography>
            {seekable
              ? "This exam session is already in progress. Click “Resume” to continue where you left off."
              : "This exam session is already in progress and cannot be resumed because it is not seekable. You can end it instead."}
          </Typography>
        ) : position === null ? (
          <Typography>
            Welcome! After you are prepared, click the &quot;Start Exam&quot;
            Button to start exam.
          </Typography>
        ) : (
          <Fragment>
            <Typography gutterBottom>
              Question {currentQuestionIndex + 1} of {numQuestions}
            </Typography>
            {/* Keying by question id resets the card's selection state every
                time a new question is served. */}
            <QuestionCard
              key={position.question.id}
              question={position.question}
            />
          </Fragment>
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
                  position === null ||
                  currentQuestionIndex === 0 ||
                  !seekable ||
                  navigate.isPending
                }
                onClick={() => goToQuestion(currentQuestionIndex - 1)}
              >
                Previous
              </Button>
            </span>
          </Tooltip>
          {interrupted ? (
            seekable ? (
              <Button
                variant="contained"
                loading={navigate.isPending}
                onClick={resumeExam}
              >
                Resume
              </Button>
            ) : (
              <Button
                variant="contained"
                color="error"
                onClick={() => setConfirmEndOpen(true)}
              >
                End Exam
              </Button>
            )
          ) : position === null ? (
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
