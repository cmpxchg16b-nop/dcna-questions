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

  const title = searchParams.get("title") ?? "";
  const shortname = searchParams.get("shortname") ?? "";
  const code = searchParams.get("code") ?? "";
  const numQuestions = Number(searchParams.get("num_questions")) || 0;
  const currentQuestionIndex =
    Number(searchParams.get("current_question_index")) || 0;

  const [selected, setSelected] = useState("");
  const [confirmEndOpen, setConfirmEndOpen] = useState(false);

  const isLastQuestion = currentQuestionIndex >= numQuestions - 1;

  // Navigates within the session by rewriting current_question_index while
  // preserving the other query params.
  const goToQuestion = (index: number) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("current_question_index", String(index));
    router.push(`/examsession?${params}`);
  };

  return (
    <Box>
      <Box sx={{ mt: 4 }}>
        <Typography variant="h4" component="h2" gutterBottom>
          {title}
        </Typography>
        <Typography gutterBottom variant="body2" color="text.secondary">
          {shortname} · {code} · {numQuestions}{" "}
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
            onClick={() => router.push("/")}
          >
            End Exam
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
