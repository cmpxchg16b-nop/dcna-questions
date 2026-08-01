"use client";

import { useState } from "react";
import {
  Box,
  Card,
  CardContent,
  Checkbox,
  FormControlLabel,
  FormGroup,
  Radio,
  RadioGroup,
  Typography,
} from "@mui/material";
import { Question } from "@/api/types";

type QuestionCardProps = {
  question: Question;
};

// QuestionCard renders one question served by GetNextQuestion. It owns the
// student's selection state; the parent keys it by question id, so the state
// resets automatically whenever a new question is served — no effects needed.
export default function QuestionCard({ question }: QuestionCardProps) {
  // selected holds the chosen option ids. Single-choice keeps a one-element
  // (or empty) array so both choice types share the same state shape.
  const [selected, setSelected] = useState<string[]>([]);

  const toggle = (optionId: string, checked: boolean) =>
    setSelected((prev) =>
      checked ? [...prev, optionId] : prev.filter((id) => id !== optionId),
    );

  return (
    <Card>
      <CardContent>
        <Typography variant="h6" component="div" gutterBottom>
          {question.description.text}
        </Typography>
        {question.exhibits?.map((exhibit) => (
          <Box
            key={exhibit.image.src}
            component="img"
            src={`/${exhibit.image.src}`}
            alt="Question exhibit"
            sx={{ display: "block", maxWidth: "100%", mb: 2 }}
          />
        ))}
        {question.type === "single-choice" && (
          <RadioGroup
            value={selected[0] ?? ""}
            onChange={(e) => setSelected([e.target.value])}
          >
            {question.options?.map((option) => (
              <FormControlLabel
                key={option.id}
                value={option.id}
                control={<Radio />}
                label={option.content}
              />
            ))}
          </RadioGroup>
        )}
        {question.type === "multiple-choice" && (
          <FormGroup>
            {question.options?.map((option) => (
              <FormControlLabel
                key={option.id}
                control={
                  <Checkbox
                    checked={selected.includes(option.id)}
                    onChange={(e) => toggle(option.id, e.target.checked)}
                  />
                }
                label={option.content}
              />
            ))}
          </FormGroup>
        )}
        {question.type !== "single-choice" &&
          question.type !== "multiple-choice" && (
            <Typography color="text.secondary">
              Questions of type &quot;{question.type}&quot; are not supported
              yet.
            </Typography>
          )}
      </CardContent>
    </Card>
  );
}
