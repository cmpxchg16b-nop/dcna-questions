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
import { useMyAnswer } from "@/hooks/useMyAnswer";
import { useSubmitAnswer } from "@/hooks/useSubmitAnswer";
import QuestionCard from "@/components/QuestionCard";
import { Assessment, ExamOptionSeekable } from "@/api/types";

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
  const submitAnswer = useSubmitAnswer(examSessionId ?? "");

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
  const started = currentQuestionIndex >= 0;
  const isPractice = session?.exam_excerpt.ExamCategory === "practice-exam";

  // myAnswer is the session's saved submission (a single exam-scoped answer,
  // so it covers every question at once). It is fetched once a question is on
  // screen: the practice-exam footer shows its "Skip (loading)" state until
  // this first fetch resolves, and new submissions are merged into it (see
  // useSubmitAnswer, which also keeps the cache current after persisting).
  const myAnswer = useMyAnswer(examSessionId, started);

  // questionState is the per-question UI state: the selected option ids and
  // the assessment revealed by the practice-exam "Check" button. It is tagged
  // with selectionKey so a new question — and, for practice exams, the first
  // resolution of my_answer — transparently starts it over: whenever the key
  // does not match, the derived values below are the fresh ones and the stale
  // state is dropped on the next update. No effects needed.
  const [questionState, setQuestionState] = useState<{
    key: string | null;
    selection: string[];
    checkResult: Assessment | null;
  }>({ key: null, selection: [], checkResult: null });

  // For practice exams the previously submitted selection is restored once
  // my_answer resolves, so the footer offers "Check" (not "Skip") for
  // questions the user already answered. Practice-exam inputs stay disabled
  // while my_answer is pending, so the restore never clobbers a selection the
  // user just made. Certification exams always start each question fresh.
  const restoredSelection =
    isPractice && effectiveQuestion
      ? (myAnswer.data?.answers
          ?.find((a) => a.questionId === effectiveQuestion.id)
          ?.options?.map((o) => o.id) ?? [])
      : [];
  const selectionKey = effectiveQuestion
    ? `${effectiveQuestion.id}:${
        isPractice ? (myAnswer.isPending ? "loading" : "loaded") : "fresh"
      }`
    : null;
  const { selection, checkResult } =
    questionState.key === selectionKey
      ? questionState
      : { selection: restoredSelection, checkResult: null };

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

  const setSelection = (sel: string[]) =>
    setQuestionState({ key: selectionKey, selection: sel, checkResult: null });

  // checkAnswer grades the current selection without persisting it
  // (check_only=true); the returned assessment reveals the correct answer and
  // turns the footer button into "Next".
  const checkAnswer = () => {
    if (!effectiveQuestion) return;
    submitAnswer.mutate(
      {
        question: effectiveQuestion,
        selectedOptionIds: selection,
        checkOnly: true,
      },
      {
        onSuccess: (result) =>
          setQuestionState({
            key: selectionKey,
            selection,
            checkResult: result.assessment,
          }),
      },
    );
  };

  // submitThenGoNext persists the current selection (checkOnly=false), then
  // navigates to the next question.
  const submitThenGoNext = () => {
    if (!effectiveQuestion) return;
    submitAnswer.mutate(
      {
        question: effectiveQuestion,
        selectedOptionIds: selection,
        checkOnly: false,
      },
      { onSuccess: () => goToQuestion(currentQuestionIndex + 1) },
    );
  };

  // submitThenConfirmEnd is the last-question counterpart of submitThenGoNext:
  // persist the current selection, then open the end-of-exam confirmation.
  const submitThenConfirmEnd = () => {
    if (!effectiveQuestion) return;
    submitAnswer.mutate(
      {
        question: effectiveQuestion,
        selectedOptionIds: selection,
        checkOnly: false,
      },
      { onSuccess: () => setConfirmEndOpen(true) },
    );
  };

  // endExam ends the exam from the last question, first persisting the
  // selection when there is one (so the last question's answer is not lost).
  const endExam = () => {
    if (selection.length > 0) {
      submitThenConfirmEnd();
    } else {
      setConfirmEndOpen(true);
    }
  };

  // inputsDisabled freezes the question's options while the saved answer is
  // loading (practice exams only, so the restore cannot clobber a fresh
  // selection), while a submission is in flight, and once the assessment is
  // on screen.
  const inputsDisabled =
    submitAnswer.isPending ||
    checkResult !== null ||
    (isPractice && myAnswer.isPending);

  // primaryButton renders the bottom-right action. Before the session starts
  // it is always "Start Exam". Once started, a practice exam runs the
  // Skip → Check → Next state machine below, while a certification exam only
  // has "Next" (disabled until something is selected), which submits with
  // checkOnly=false and moves on without revealing anything. On the last
  // question "Next" is replaced by "End Exam" in the same slot.
  const primaryButton = () => {
    if (!started) {
      return (
        <Button
          variant="contained"
          loading={navigate.isPending}
          onClick={startExam}
        >
          Start Exam
        </Button>
      );
    }
    if (!effectiveQuestion) return null;
    if (isPractice) {
      if (myAnswer.isPending) {
        // Confirming whether the current question was already answered.
        return (
          <Button variant="contained" loading disabled>
            Skip
          </Button>
        );
      }
      if (checkResult) {
        // The assessment is on screen, so the only way forward is submitting
        // the selection for real (checkOnly=false) and moving on.
        return isLastQuestion ? (
          <Button
            variant="contained"
            color="error"
            loading={submitAnswer.isPending}
            onClick={submitThenConfirmEnd}
          >
            End Exam
          </Button>
        ) : (
          <Button
            variant="contained"
            loading={submitAnswer.isPending || navigate.isPending}
            onClick={submitThenGoNext}
          >
            Next
          </Button>
        );
      }
      if (selection.length > 0) {
        return (
          <Button
            variant="contained"
            loading={submitAnswer.isPending}
            onClick={checkAnswer}
          >
            Check
          </Button>
        );
      }
      // Nothing selected and nothing previously submitted: Skip moves on
      // without submitting anything; on the last question the way "on" is
      // ending the exam.
      return isLastQuestion ? (
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
          Skip
        </Button>
      );
    }
    // Certification exam: no answer reveal, submissions always persist.
    if (isLastQuestion) {
      return (
        <Button
          variant="contained"
          color="error"
          loading={submitAnswer.isPending}
          onClick={endExam}
        >
          End Exam
        </Button>
      );
    }
    return (
      <Button
        variant="contained"
        disabled={selection.length === 0}
        loading={submitAnswer.isPending || navigate.isPending}
        onClick={submitThenGoNext}
      >
        Next
      </Button>
    );
  };

  return (
    <Box>
      <Box sx={{ mt: 4 }}>
        <Typography variant="h4" component="h2" gutterBottom>
          {excerpt.Title}
        </Typography>
        <Typography gutterBottom variant="body2" color="textSecondary">
          {excerpt.ShortName} · {excerpt.Code} · {numQuestions}{" "}
          {numQuestions === 1 ? "question" : "questions"}
        </Typography>
      </Box>

      <Box sx={{ mt: 4 }}>
        {!started ? (
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
              <QuestionCard
                key={effectiveQuestion.id}
                question={effectiveQuestion}
                selected={selection}
                onSelectionChange={setSelection}
                disabled={inputsDisabled}
                assessment={checkResult}
              />
            </Fragment>
          )
        )}

        {(navigate.isError || submitAnswer.isError || myAnswer.isError) && (
          <Typography color="error" sx={{ mt: 1 }}>
            {navigate.error?.message ??
              submitAnswer.error?.message ??
              myAnswer.error?.message}
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
          {primaryButton()}
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
