"use client";

import {
  Box,
  Button,
  Card,
  CardContent,
  ListItem,
  Typography,
} from "@mui/material";
import { ExamExcerpt } from "@/api/types";
import ExamMetadataChips from "@/components/ExamMetadataChips";

type ExamCardProps = {
  exam: ExamExcerpt;
  onTake: (exam: ExamExcerpt) => void;
};

// One entry in the Exams list: the exam's metadata and a two-line clamped
// description, with a Take action reported via onTake so the parent can
// collect exam options before creating the session.
export default function ExamCard({ exam, onTake }: ExamCardProps) {
  return (
    <ListItem disableGutters sx={{ mb: 1 }}>
      <Card sx={{ width: "100%" }}>
        <CardContent>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            {/* minWidth: 0 lets the text column shrink so the clamp can kick in
              instead of pushing the button off-card. */}
            <Box sx={{ flexGrow: 1, minWidth: 0 }}>
              <Typography variant="h6" component="div" noWrap>
                {exam.Title}
              </Typography>
              <ExamMetadataChips exam={exam} />
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{
                  display: "-webkit-box",
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: "vertical",
                  overflow: "hidden",
                }}
              >
                {exam.Description}
              </Typography>
            </Box>
            <Button variant="contained" onClick={() => onTake(exam)}>
              Take
            </Button>
          </Box>
        </CardContent>
      </Card>
    </ListItem>
  );
}
