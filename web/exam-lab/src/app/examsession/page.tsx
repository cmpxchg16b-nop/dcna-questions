"use client";

import { Fragment, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Box,
  Button,
  Card,
  CardContent,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
  Radio,
  RadioGroup,
  Typography,
} from "@mui/material";
import { useExamSession } from "@/hooks/useExamSession";
import { useEndExamSession } from "@/hooks/useEndExamSession";

// Mocked single-choice question, mirroring the Go question.Question shape
// (pkg/models/question/question.go) until the session API serves real ones.
const mockQuestion = {
  id: "q1",
  type: "single-choice",
  description:
    "Which protocol is used to dynamically assign IP addresses to hosts on a network?",
  options: [
    { id: "a", content: "DNS" },
    { id: "b", content: "DHCP" },
    { id: "c", content: "NAT" },
    { id: "d", content: "ARP" },
  ],
};

export default function Page() {
  const router = useRouter();
  const searchParams = useSearchParams();

  // The page is keyed off the session id alone; the session's metadata (title,
  // shortname, code, total question count) and current question index are all
  // sourced from the server rather than query params.
  const examSessionId = searchParams.get("exam_session_id");

  const { data: session, isPending, isError } = useExamSession(examSessionId);
  const endSession = useEndExamSession();

  const [selected, setSelected] = useState("");
  const [confirmEndOpen, setConfirmEndOpen] = useState(false);

  const serverIndex = session?.current_question_index ?? -1;
  const currentQuestionIndex = serverIndex;

  const numQuestions = session?.exam_excerpt.NumQuestions ?? 0;
  const isLastQuestion = currentQuestionIndex >= numQuestions - 1;
  const cursorRef = useRef<string>("");

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
      // go next
      // todo:
      // 1. call api to get next question using the cursor in cursorRef, it's fine even if cursorRef currently stores an empty cursor
      // 2. update the cursorRef with the server returned cursor at step 2
    } else if (nextIndex === currentQuestionIndex - 1) {
      // go previous
      // todo:
      // 1. call api to seek cursor to previous index, update the cursorRef with the server returned cursor
      // 2. call api to get next question using the cursor obtained at step 1
      // 3. update the cursorRef with the server returned cursor at step 2
    } else {
      // unsupported
      console.error(`Unsupported index: ${nextIndex}`);
    }
  };

  const startExam = () => {
    if (currentQuestionIndex !== -1) {
      console.error("Cannot start exam: already in progress");
      return;
    } else {
      goToQuestion(0);
    }
  };

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
          <Fragment>
            {/* Todo: Render the question using real server-side data (obtained by calling GetNextQuestion() ) */}
            <Typography gutterBottom>
              Question {currentQuestionIndex + 1} of {numQuestions}
            </Typography>
            <Card>
              <CardContent>
                <Typography variant="h6" component="div" gutterBottom>
                  {mockQuestion.description}
                </Typography>
                <RadioGroup
                  value={selected}
                  onChange={(e) => setSelected(e.target.value)}
                >
                  {mockQuestion.options.map((option) => (
                    <FormControlLabel
                      key={option.id}
                      value={option.id}
                      control={<Radio />}
                      label={option.content}
                    />
                  ))}
                </RadioGroup>
              </CardContent>
            </Card>
          </Fragment>
        )}

        {/* End Exam replaces Next in the same flex slot on the last question,
            so it occupies exactly the same position. */}
        <Box sx={{ display: "flex", justifyContent: "space-between", mt: 2 }}>
          <Button
            variant="contained"
            disabled={currentQuestionIndex === 0}
            onClick={() => goToQuestion(currentQuestionIndex - 1)}
          >
            Previous
          </Button>
          {isLastQuestion ? (
            <Button
              variant="contained"
              color="error"
              onClick={() => setConfirmEndOpen(true)}
            >
              End Exam
            </Button>
          ) : currentQuestionIndex === -1 ? (
            <Button variant="contained" onClick={() => startExam()}>
              Start Exam
            </Button>
          ) : (
            <Button
              variant="contained"
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
