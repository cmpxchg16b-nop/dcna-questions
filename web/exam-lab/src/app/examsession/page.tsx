"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import {
  Box,
  Card,
  CardContent,
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
  const searchParams = useSearchParams();

  const title = searchParams.get("title") ?? "";
  const shortname = searchParams.get("shortname") ?? "";
  const code = searchParams.get("code") ?? "";
  const numQuestions = Number(searchParams.get("num_questions")) || 0;
  const currentQuestionIndex =
    Number(searchParams.get("current_question_index")) || 0;

  const [selected, setSelected] = useState("");

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
      </Box>
    </Box>
  );
}
