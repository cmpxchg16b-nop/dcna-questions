"use client";

import { useState } from "react";
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

  // currentQuestionIndex is seeded from the server's value once the session has
  // loaded. Local Next/Previous navigation adjusts it client-side; advancing the
  // server's index (via GetNextQuestion) is wired up separately.
  const [indexOverride, setIndexOverride] = useState<number | null>(null);
  const serverIndex = session?.current_question_index ?? -1;
  const currentQuestionIndex =
    indexOverride !== null ? indexOverride : serverIndex;

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
  const goToQuestion = (index: number) => setIndexOverride(index);

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
