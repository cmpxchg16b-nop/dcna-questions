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
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import { useExamSession } from "@/hooks/useExamSession";
import { useEndExamSession } from "@/hooks/useEndExamSession";
import { useNavigateQuestion } from "@/hooks/useNavigateQuestion";
import { useMyAnswer } from "@/hooks/useMyAnswer";
import { useSubmitAnswer } from "@/hooks/useSubmitAnswer";
import QuestionCard from "@/components/QuestionCard";
import { Assessment, Connect, ExamOptionSeekable } from "@/api/types";
import { useTranslation } from "react-i18next";

export default function Page() {
  const { t } = useTranslation();
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
  // gotoOpen/gotoValue drive the "Goto" dialog: gotoValue is the raw text of
  // the question-number field (a string so partial input is not clobbered).
  const [gotoOpen, setGotoOpen] = useState(false);
  const [gotoValue, setGotoValue] = useState("");

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

  // questionState is the per-question UI state: the selected option ids
  // (choice questions), the placed connections (drag-and-drop questions), and
  // the assessment revealed by the practice-exam "Check" button. It is tagged
  // with selectionKey so a new question — and, for practice exams, the first
  // resolution of my_answer — transparently starts it over: whenever the key
  // does not match, the derived values below are the fresh ones and the stale
  // state is dropped on the next update. No effects needed.
  const [questionState, setQuestionState] = useState<{
    key: string | null;
    selection: string[];
    connections: Connect[];
    checkResult: Assessment | null;
  }>({ key: null, selection: [], connections: [], checkResult: null });

  // For practice exams the previously submitted answer is restored once
  // my_answer resolves, so the footer offers "Check" (not "Skip") for
  // questions the user already answered. Practice-exam inputs stay disabled
  // while my_answer is pending, so the restore never clobbers a selection the
  // user just made. Certification exams always start each question fresh.
  const restoredAnswer =
    isPractice && effectiveQuestion
      ? myAnswer.data?.answers?.find(
          (a) => a.questionId === effectiveQuestion.id,
        )
      : undefined;
  const restoredSelection = restoredAnswer?.options?.map((o) => o.id) ?? [];
  const restoredConnections = restoredAnswer?.connections ?? [];
  const selectionKey = effectiveQuestion
    ? `${effectiveQuestion.id}:${
        isPractice ? (myAnswer.isPending ? "loading" : "loaded") : "fresh"
      }`
    : null;
  const { selection, connections, checkResult } =
    questionState.key === selectionKey
      ? questionState
      : {
          selection: restoredSelection,
          connections: restoredConnections,
          checkResult: null,
        };
  // hasAnswer is the footer's "is there anything to submit" test: chosen
  // options for the choice types, placed connections for drag-and-drop.
  const hasAnswer = selection.length > 0 || connections.length > 0;

  if (!examSessionId) {
    return (
      <Box sx={{ mt: 4 }}>
        <Typography color="error">{t("session.missingId")}</Typography>
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
          {t("session.loadFailed", { sessionId: examSessionId })}
        </Typography>
      </Box>
    );
  }

  const excerpt = session.exam_excerpt;

  const goToQuestion = (nextIndex: number) => {
    if (nextIndex < 0 || nextIndex >= numQuestions) {
      // out of range
      console.error(`Unsupported index: ${nextIndex}`);
      return;
    }
    if (nextIndex === currentQuestionIndex + 1) {
      // go next: read through the cursor tracked by the navigation hook.
      navigate.mutate({ index: nextIndex, seek: false });
    } else {
      // Any other move (Previous, or an arbitrary Goto jump) repositions the
      // cursor to the target index (requires a seekable session), then reads
      // the question there.
      navigate.mutate({ index: nextIndex, seek: true });
    }
  };

  const startExam = () => goToQuestion(0);

  // The Goto dialog asks for a 1-based question number; gotoNumber/gotoValid
  // are its parsed value and range check against the exam's question count.
  const gotoNumber = Number(gotoValue);
  const gotoValid =
    gotoValue.trim() !== "" &&
    Number.isInteger(gotoNumber) &&
    gotoNumber >= 1 &&
    gotoNumber <= numQuestions;
  const confirmGoto = () => {
    if (!gotoValid) return;
    setGotoOpen(false);
    goToQuestion(gotoNumber - 1);
  };

  const setSelection = (sel: string[]) =>
    setQuestionState({
      key: selectionKey,
      selection: sel,
      connections,
      checkResult: null,
    });

  const setConnections = (conns: Connect[]) =>
    setQuestionState({
      key: selectionKey,
      selection,
      connections: conns,
      checkResult: null,
    });

  // checkAnswer grades the current selection without persisting it
  // (check_only=true); the returned assessment reveals the correct answer and
  // turns the footer button into "Next".
  const checkAnswer = () => {
    if (!effectiveQuestion) return;
    submitAnswer.mutate(
      {
        question: effectiveQuestion,
        selectedOptionIds: selection,
        connections,
        checkOnly: true,
      },
      {
        onSuccess: (result) =>
          setQuestionState({
            key: selectionKey,
            selection,
            connections,
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
        connections,
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
        connections,
        checkOnly: false,
      },
      { onSuccess: () => setConfirmEndOpen(true) },
    );
  };

  // endExam ends the exam from the last question, first persisting the
  // selection when there is one (so the last question's answer is not lost).
  const endExam = () => {
    if (hasAnswer) {
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
          {t("session.start")}
        </Button>
      );
    }
    if (!effectiveQuestion) return null;
    if (isPractice) {
      if (myAnswer.isPending) {
        // Confirming whether the current question was already answered.
        return (
          <Button variant="contained" loading disabled>
            {t("session.skip")}
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
            {t("session.end")}
          </Button>
        ) : (
          <Button
            variant="contained"
            loading={submitAnswer.isPending || navigate.isPending}
            onClick={submitThenGoNext}
          >
            {t("session.next")}
          </Button>
        );
      }
      if (hasAnswer) {
        return (
          <Button
            variant="contained"
            loading={submitAnswer.isPending}
            onClick={checkAnswer}
          >
            {t("session.check")}
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
          {t("session.end")}
        </Button>
      ) : (
        <Button
          variant="contained"
          loading={navigate.isPending}
          onClick={() => goToQuestion(currentQuestionIndex + 1)}
        >
          {t("session.skip")}
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
          {t("session.end")}
        </Button>
      );
    }
    return (
      <Button
        variant="contained"
        disabled={!hasAnswer}
        loading={submitAnswer.isPending || navigate.isPending}
        onClick={submitThenGoNext}
      >
        {t("session.next")}
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
          {excerpt.ShortName} · {excerpt.Code} ·{" "}
          {t("exam.questionCount", { count: numQuestions })}
        </Typography>
      </Box>

      <Box sx={{ mt: 4 }}>
        {!started ? (
          <Typography>{t("session.welcome")}</Typography>
        ) : (
          effectiveQuestion && (
            <Fragment>
              <QuestionCard
                key={effectiveQuestion.id}
                question={effectiveQuestion}
                questionNumber={currentQuestionIndex + 1}
                selected={selection}
                onSelectionChange={setSelection}
                connections={connections}
                onConnectionsChange={setConnections}
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
          <Box sx={{ display: "flex", gap: 1 }}>
            <Tooltip title={seekable ? "" : t("session.notSeekable")}>
              {/* MUI Tooltips don't fire on disabled buttons, hence the span. */}
              <span>
                <Button
                  variant="contained"
                  disabled={
                    currentQuestionIndex <= 0 || !seekable || navigate.isPending
                  }
                  onClick={() => goToQuestion(currentQuestionIndex - 1)}
                >
                  {t("session.previous")}
                </Button>
              </span>
            </Tooltip>
            {/* Goto repositions the cursor just like Previous, so it has the
                same seekability requirement. It is also disabled on the
                welcome page: there is nothing to jump between until the exam
                has started. */}
            <Tooltip title={seekable ? "" : t("session.notSeekable")}>
              <span>
                <Button
                  variant="contained"
                  disabled={!started || !seekable || navigate.isPending}
                  onClick={() => {
                    // Prefill with the current question number (1-based).
                    setGotoValue(String(Math.max(currentQuestionIndex + 1, 1)));
                    setGotoOpen(true);
                  }}
                >
                  {t("session.goto")}
                </Button>
              </span>
            </Tooltip>
          </Box>
          {primaryButton()}
        </Box>
      </Box>

      {/* Jump-to-question dialog: a 1-based question number, clamped to the
          exam's question count. Enter confirms, Escape dismisses. fullWidth
          keeps the dialog at the xs breakpoint width instead of shrinking to
          fit the number field. */}
      <Dialog
        open={gotoOpen}
        onClose={() => setGotoOpen(false)}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>{t("session.gotoTitle")}</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            type="number"
            margin="dense"
            label={t("session.gotoLabel", { count: numQuestions })}
            value={gotoValue}
            onChange={(e) => setGotoValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") confirmGoto();
            }}
            error={gotoValue.trim() !== "" && !gotoValid}
            helperText={
              gotoValue.trim() !== "" && !gotoValid
                ? t("session.gotoInvalid", { count: numQuestions })
                : " "
            }
            slotProps={{ htmlInput: { min: 1, max: numQuestions, step: 1 } }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setGotoOpen(false)}>
            {t("common.cancel")}
          </Button>
          <Button
            variant="contained"
            disabled={!gotoValid}
            loading={navigate.isPending}
            onClick={confirmGoto}
          >
            {t("session.goto")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={confirmEndOpen} onClose={() => setConfirmEndOpen(false)}>
        <DialogTitle>{t("session.endConfirmTitle")}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t("session.endConfirmBody")}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmEndOpen(false)}>
            {t("common.cancel")}
          </Button>
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
            {t("session.end")}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
