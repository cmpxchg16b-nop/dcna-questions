"use client";

import {
  Box,
  Card,
  CardContent,
  Chip,
  ListItem,
  Tooltip,
  Typography,
} from "@mui/material";
import { formatDistanceToNow } from "date-fns";
import { ExamCategoryLabels, ExamReport } from "@/api/types";

type ExamReportCardProps = {
  report: ExamReport;
};

// One entry in the Trackings list: a finished exam's report rendered as a
// score card — the exam's metadata and finish time on the left, the graded
// result (pass/fail verdict and earned/total score) on the right. Unlike
// ExamMetadataChips the report carries no question count, so the chips are
// rendered here directly.
export default function ExamReportCard({ report }: ExamReportCardProps) {
  const { assessment } = report;
  const score = assessment.scoreResult;
  // overallResult is "pass" or "immediate" (any non-passing verdict), so it
  // renders as a Pass/Fail chip.
  const passed = assessment.overallResult === "pass";
  return (
    <ListItem disableGutters sx={{ mb: 1 }}>
      <Card sx={{ width: "100%" }}>
        <CardContent>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <Box sx={{ flexGrow: 1, minWidth: 0 }}>
              <Typography variant="h6" component="div" noWrap>
                {report.title}
              </Typography>
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mb: 1 }}>
                {report.examShortName && (
                  <Chip label={report.examShortName} size="small" />
                )}
                {report.examCode && (
                  <Chip label={report.examCode} size="small" />
                )}
                <Chip
                  label={ExamCategoryLabels[report.examCategory]}
                  size="small"
                />
              </Box>
              <Typography variant="body2" color="text.secondary">
                Finished{" "}
                <Tooltip title={new Date(report.finishedAt).toLocaleString()}>
                  <Box component="span">
                    {formatDistanceToNow(new Date(report.finishedAt), {
                      addSuffix: true,
                    })}
                  </Box>
                </Tooltip>
              </Typography>
            </Box>
            {assessment.overallResult && (
              <Chip
                label={passed ? "Pass" : "Fail"}
                color={passed ? "success" : "error"}
              />
            )}
            {score && (
              <Box sx={{ textAlign: "center", whiteSpace: "nowrap" }}>
                <Typography variant="h5" component="div">
                  {score.earnedScore}/{score.totalScore}
                </Typography>
                {report.passingScore != null && (
                  <Typography variant="body2" color="text.secondary">
                    pass at {report.passingScore}
                  </Typography>
                )}
              </Box>
            )}
          </Box>
        </CardContent>
      </Card>
    </ListItem>
  );
}
