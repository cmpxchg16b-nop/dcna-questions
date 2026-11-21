"use client";

import {
  Alert,
  Box,
  Card,
  CardContent,
  Checkbox,
  FormControlLabel,
  FormGroup,
  Link,
  Radio,
  RadioGroup,
  Typography,
} from "@mui/material";
import { Assessment, Connect, Question, QuestionOption } from "@/api/types";
import { isConnectionAnswerCorrect } from "@/api/dragAndDrop";
import { resolveAssetSrc } from "@/utils";
import DragAndDropBoard from "./DragAndDropBoard";
import ImgDragAndDropBoard from "./ImgDragAndDropBoard";
import { useTranslation } from "react-i18next";

type QuestionCardProps = {
  question: Question;
  // questionNumber is the 1-based position of the question in the exam,
  // rendered as a "N. " prefix on the description.
  questionNumber: number;
  // selected holds the chosen option ids. The card is controlled so the page
  // can drive the Check/Next/Skip footer state, restore previously submitted
  // selections, and reset the selection per question. Single-choice keeps a
  // one-element (or empty) array so both choice types share the same shape.
  selected: string[];
  onSelectionChange: (selected: string[]) => void;
  // connections holds the placed candidate→drop connections for a
  // drag-and-drop question, playing the same controlled role as selected.
  connections: Connect[];
  onConnectionsChange: (connections: Connect[]) => void;
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
  questionNumber,
  selected,
  onSelectionChange,
  connections,
  onConnectionsChange,
  disabled = false,
  assessment = null,
}: QuestionCardProps) {
  const { t } = useTranslation();
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
  // Correctness mirrors the grader's semantics: single-choice is correct when
  // exactly one option is chosen and it is among the correct ones;
  // multiple-choice requires an exact option-set match; drag-and-drop requires
  // satisfying one of the question's connection solutions. It stays undefined
  // unless the assessment attached this question's origin document with the
  // relevant correct-answer data (practice exams only).
  const connectionSolutions =
    gradedQuestion?.correctAnswer?.connectionSolutions ?? [];
  const isCorrect = gradedQuestion
    ? question.type === "single-choice"
      ? selected.length === 1 && correctIds.has(selected[0])
      : question.type === "multiple-choice"
        ? correctIds.size === selected.length &&
          selected.every((id) => correctIds.has(id))
        : question.type === "drag-and-drop" && connectionSolutions.length > 0
          ? isConnectionAnswerCorrect(connections, connectionSolutions)
          : undefined
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
            ? t("question.yourAnswerCorrect")
            : t("question.correctAnswer")
          : t("question.yourAnswerIncorrect")}
      </Typography>
    );
  };

  // optionLabel renders an option's content and, when imgSrc is set, the
  // option's image (pure-image options carry no text). Its rationale comes
  // from the graded origin by option id, so randomized option order is safe.
  // The served question has no rationales; they appear only after Check.
  const optionLabel = (option: QuestionOption) => {
    const explanation = assessment
      ? gradedQuestion?.options?.find((o) => o.id === option.id)?.explanation
      : undefined;
    return (
      <>
        {option.content}
        {option.imgSrc && (
          <Box
            component="img"
            src={resolveAssetSrc(option.imgSrc)}
            alt={option.content || t("question.optionImageAlt")}
            sx={{
              display: "block",
              maxWidth: "100%",
              maxHeight: "40vh",
              my: 0.5,
            }}
          />
        )}
        {optionMarker(option.id)}
        {explanation && (
          <Typography
            component="span"
            variant="body2"
            color="textSecondary"
            data-testid={`option-explanation-${option.id}`}
            sx={{ display: "block", mt: 0.75, whiteSpace: "pre-wrap" }}
          >
            {explanation}
          </Typography>
        )}
      </>
    );
  };

  return (
    <Card>
      <CardContent>
        <Typography gutterBottom>
          {questionNumber}. {question.description.text}
        </Typography>
        {question.exhibits?.map((exhibit, idx) => (
          // maxHeight caps tall exhibits at a fraction of the viewport so
          // they don't push the answer options below the fold; with only max
          // bounds set, the image scales down proportionally. The key pairs
          // the index with the src since two exhibits may share one image.
          <Box
            key={`${idx}:${exhibit.image.src}`}
            component="img"
            src={resolveAssetSrc(exhibit.image.src)}
            alt={t("question.exhibitAlt")}
            sx={{
              display: "block",
              maxWidth: "100%",
              maxHeight: "40vh",
              mb: 2,
            }}
          />
        ))}
        {assessment && isCorrect !== undefined && (
          <Alert severity={isCorrect ? "success" : "error"} sx={{ mb: 2 }}>
            {isCorrect ? t("question.correct") : t("question.incorrect")}
          </Alert>
        )}
        {question.type === "single-choice" && (
          // gap opens up breathing room between options; it matters most on
          // narrow mobile screens where option text wraps onto several lines.
          <RadioGroup
            value={selected[0] ?? ""}
            onChange={(e) => onSelectionChange([e.target.value])}
            sx={{ gap: { xs: 1.5, sm: 1 } }}
          >
            {question.options?.map((option) => (
              <FormControlLabel
                key={option.id}
                value={option.id}
                control={<Radio />}
                label={optionLabel(option)}
                disabled={disabled}
                sx={{ alignItems: "flex-start" }}
              />
            ))}
          </RadioGroup>
        )}
        {question.type === "multiple-choice" && (
          <FormGroup sx={{ gap: { xs: 1.5, sm: 1 } }}>
            {question.options?.map((option) => (
              <FormControlLabel
                key={option.id}
                control={
                  <Checkbox
                    checked={selected.includes(option.id)}
                    onChange={(e) => toggle(option.id, e.target.checked)}
                  />
                }
                label={optionLabel(option)}
                disabled={disabled}
                sx={{ alignItems: "flex-start" }}
              />
            ))}
          </FormGroup>
        )}
        {question.type === "drag-and-drop" &&
          (question.imgDragAndDrop ? (
            <ImgDragAndDropBoard
              question={question}
              connections={connections}
              onConnectionsChange={onConnectionsChange}
              disabled={disabled}
              assessment={assessment}
            />
          ) : (
            <DragAndDropBoard
              question={question}
              connections={connections}
              onConnectionsChange={onConnectionsChange}
              disabled={disabled}
              assessment={assessment}
            />
          ))}
        {question.type !== "single-choice" &&
          question.type !== "multiple-choice" &&
          question.type !== "drag-and-drop" && (
            <Typography color="textSecondary">
              {t("question.unsupportedType", { type: question.type })}
            </Typography>
          )}
        {assessment &&
          gradedQuestion &&
          (gradedQuestion.explanation ||
            (gradedQuestion.references?.length ?? 0) > 0) && (
            <Box
              component="section"
              aria-label={t("question.explanation")}
              data-testid="question-feedback"
              sx={{ mt: 2, p: 2, border: 1, borderColor: "divider", borderRadius: 1 }}
            >
              {gradedQuestion.explanation && (
                <>
                  <Typography variant="subtitle2" gutterBottom>
                    {t("question.explanation")}
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{
                      mb: (gradedQuestion.references?.length ?? 0) > 0 ? 2 : 0,
                      whiteSpace: "pre-wrap",
                    }}
                  >
                    {gradedQuestion.explanation}
                  </Typography>
                </>
              )}
              {gradedQuestion.references &&
                gradedQuestion.references.length > 0 && (
                  <>
                    <Typography variant="subtitle2" gutterBottom>
                      {t("question.references")}
                    </Typography>
                    <Box component="ul" sx={{ m: 0, pl: 3 }}>
                      {gradedQuestion.references.map((reference, index) => (
                        <Box
                          component="li"
                          key={`${reference.href}:${index}`}
                          sx={{ overflowWrap: "anywhere" }}
                        >
                          {/^https?:\/\//i.test(reference.href) ? (
                            <Link
                              href={reference.href}
                              target="_blank"
                              rel="noopener noreferrer"
                              variant="body2"
                            >
                              {reference.title || reference.href}
                            </Link>
                          ) : (
                            <Typography variant="body2">
                              {reference.title || reference.href}
                            </Typography>
                          )}
                        </Box>
                      ))}
                    </Box>
                  </>
                )}
            </Box>
          )}
      </CardContent>
    </Card>
  );
}
