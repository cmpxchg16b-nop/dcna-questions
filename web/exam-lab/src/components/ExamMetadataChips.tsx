"use client";

import { Box, Chip } from "@mui/material";
import { ExamExcerpt } from "@/api/types";
import { useTranslation } from "react-i18next";

type ExamMetadataChipsProps = {
  exam: ExamExcerpt;
};

// An exam's metadata — short name, code, category, and question count — as a
// row of small chips, shared by the exam and exam session cards.
export default function ExamMetadataChips({ exam }: ExamMetadataChipsProps) {
  const { t } = useTranslation();
  return (
    <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mb: 1 }}>
      <Chip label={exam.ShortName} size="small" />
      <Chip label={exam.Code} size="small" />
      <Chip label={t(`exam.category.${exam.ExamCategory}`)} size="small" />
      {/* i18next picks the plural form for the active language from count,
          so no manual singular/plural branch is needed. */}
      <Chip
        label={t("exam.questionCount", { count: exam.NumQuestions })}
        size="small"
      />
    </Box>
  );
}
