"use client";

import { Box, Chip } from "@mui/material";
import { ExamCategoryLabels, ExamExcerpt } from "@/api/types";

type ExamMetadataChipsProps = {
  exam: ExamExcerpt;
};

// An exam's metadata — short name, code, category, and question count — as a
// row of small chips, shared by the exam and exam session cards.
export default function ExamMetadataChips({ exam }: ExamMetadataChipsProps) {
  const questions =
    exam.NumQuestions === 1 ? "1 question" : `${exam.NumQuestions} questions`;
  return (
    <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mb: 1 }}>
      <Chip label={exam.ShortName} size="small" />
      <Chip label={exam.Code} size="small" />
      <Chip label={ExamCategoryLabels[exam.ExamCategory]} size="small" />
      <Chip label={questions} size="small" />
    </Box>
  );
}
