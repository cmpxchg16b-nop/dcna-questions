"use client";

import {
  Alert,
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
import { Assessment, Question } from "@/api/types";

type QuestionCardProps = {
  question: Question;
  // selected holds the chosen option ids. The card is controlled so the page
  // can drive the Check/Next/Skip footer state, restore previously submitted
  // selections, and reset the selection per question. Single-choice keeps a
  // one-element (or empty) array so both choice types share the same shape.
  selected: string[];
  onSelectionChange: (selected: string[]) => void;
  // disabled freezes the inputs while the saved answer is loading, while a
  // submission is in flight, and once the assessment is on screen.
  disabled?: boolean;
  // assessment is the practice-exam "Check" result for this question; when
  // present the card reveals the correct answer alongside the candidate's own.
  assessment?: Assessment | null;
};

// QuestionCard renders one question served by GetNextQuestion together with
// the student's selection. When an assessment is provided (practice exam,
// after "Check"), it also marks each option: the correct answer in green, the
// candidate's own wrong picks in red.
export default function QuestionCard({
  question,
  selected,
  onSelectionChange,
  disabled = false,
  assessment = null,
}: QuestionCardProps) {
  const toggle = (optionId: string, checked: boolean) =>
    onSelectionChange(
      checked
        ? [...selected, optionId]
        : selected.filter((id) => id !== optionId),
    );

  // gradedQuestion is this question's origin document inside the assessment
  // (attached by the grader for practice-exam submissions only); its
  // correctAnswer drives the option markers below.
  const gradedQuestion = assessment?.questions?.find(
    (q) => q.id === question.id,
  );
  const correctIds = new Set(
    gradedQuestion?.correctAnswer?.options?.map((o) => o.id) ?? [],
  );
  // Correctness mirrors the grader's option-set semantics: single-choice is
  // correct when exactly one option is chosen and it is among the correct
  // ones; multiple-choice requires an exact set match.
  const isCorrect = gradedQuestion
    ? question.type === "single-choice"
      ? selected.length === 1 && correctIds.has(selected[0])
      : correctIds.size === selected.length &&
        selected.every((id) => correctIds.has(id))
    : undefined;

  // optionMarker renders the per-option verdict once the assessment is on
  // screen: "Correct answer" in green, the candidate's own answer in red when
  // it is wrong — or both, in green, when they coincide. Typography's color
  // prop only recognizes MUI v9's simple palette keys ("success", "error",
  // "textSecondary", ...) — dotted paths like "success.main" emit no style,
  // and the marker sits inside a disabled FormControlLabel, so without a real
  // color it inherits the dimmed text.disabled gray.
  const optionMarker = (optionId: string) => {
    if (!assessment) return null;
    const isCorrectOption = correctIds.has(optionId);
    const isMine = selected.includes(optionId);
    if (!isCorrectOption && !isMine) return null;
    return (
      <Typography
        component="span"
        variant="caption"
        color={isCorrectOption ? "success" : "error"}
        sx={{ ml: 1, fontWeight: 600 }}
      >
        {isCorrectOption
          ? isMine
            ? "Your answer — correct"
            : "Correct answer"
          : "Your answer — incorrect"}
      </Typography>
    );
  };

  return (
    <Card>
      <CardContent>
        <Typography gutterBottom>{question.description.text}</Typography>
        {question.exhibits?.map((exhibit) => (
          <Box
            key={exhibit.image.src}
            component="img"
            src={`/${exhibit.image.src}`}
            alt="Question exhibit"
            sx={{ display: "block", maxWidth: "100%", mb: 2 }}
          />
        ))}
        {assessment && isCorrect !== undefined && (
          <Alert severity={isCorrect ? "success" : "error"} sx={{ mb: 2 }}>
            {isCorrect
              ? "Correct!"
              : "Incorrect — the correct answer is marked below."}
          </Alert>
        )}
        {question.type === "single-choice" && (
          <RadioGroup
            value={selected[0] ?? ""}
            onChange={(e) => onSelectionChange([e.target.value])}
          >
            {question.options?.map((option) => (
              <FormControlLabel
                key={option.id}
                value={option.id}
                control={<Radio />}
                label={
                  <>
                    {option.content}
                    {optionMarker(option.id)}
                  </>
                }
                disabled={disabled}
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
                label={
                  <>
                    {option.content}
                    {optionMarker(option.id)}
                  </>
                }
                disabled={disabled}
              />
            ))}
          </FormGroup>
        )}
        {question.type !== "single-choice" &&
          question.type !== "multiple-choice" && (
            <Typography color="textSecondary">
              Questions of type &quot;{question.type}&quot; are not supported
              yet.
            </Typography>
          )}
      </CardContent>
    </Card>
  );
}
