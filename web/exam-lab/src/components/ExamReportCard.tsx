"use client";

import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  ListItem,
  Tooltip,
  Typography,
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import { formatDistanceToNow } from "date-fns";
import { ExamCategoryLabels, ExamReport } from "@/api/types";

type ExamReportCardProps = {
  report: ExamReport;
  onDelete: (report: ExamReport) => void;
};

// collapsibleButtonSx makes an action button collapse to an icon-only square
// below the sm breakpoint: the label is hidden (see labelSx) and the start
// icon's spacing is neutralized so the icon stays centered.
const collapsibleButtonSx = {
  whiteSpace: "nowrap",
  minWidth: { xs: 0, sm: 64 },
  px: { xs: 1, sm: 2 },
  "& .MuiButton-startIcon": {
    ml: { xs: 0, sm: "-4px" },
    mr: { xs: 0, sm: "8px" },
  },
};

// labelSx hides a button's text label below the sm breakpoint, paired with
// collapsibleButtonSx. The text stays in the DOM at wider widths; the button
// carries an aria-label so it remains accessible when the text is hidden.
const labelSx = { display: { xs: "none", sm: "inline" } };

// One entry in the Trackings list: a finished exam's report rendered as a
// score card — the exam's metadata and finish time on the left, the graded
// result (pass/fail verdict and earned/total score) on the right, plus a
// Delete action. Delete is reported via onDelete so the parent can ask for
// confirmation first. Unlike ExamMetadataChips the report carries no question
// count, so the chips are rendered here directly.
export default function ExamReportCard({
  report,
  onDelete,
}: ExamReportCardProps) {
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
              <Typography variant="body2" color="textSecondary">
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
                  <Typography variant="body2" color="textSecondary">
                    pass at {report.passingScore}
                  </Typography>
                )}
              </Box>
            )}
            <Button
              variant="contained"
              color="error"
              sx={collapsibleButtonSx}
              aria-label="Delete"
              startIcon={<DeleteIcon />}
              onClick={() => onDelete(report)}
            >
              <Box component="span" sx={labelSx}>
                Delete
              </Box>
            </Button>
          </Box>
        </CardContent>
      </Card>
    </ListItem>
  );
}
