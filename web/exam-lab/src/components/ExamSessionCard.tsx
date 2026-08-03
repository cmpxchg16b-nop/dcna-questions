"use client";

import {
  Box,
  Button,
  Card,
  CardContent,
  ListItem,
  Tooltip,
  Typography,
} from "@mui/material";
import { formatDistanceToNow } from "date-fns";
import { useRouter } from "next/navigation";
import { ExamSessionSummary } from "@/api/types";
import ExamMetadataChips from "@/components/ExamMetadataChips";

type ExamSessionCardProps = {
  session: ExamSessionSummary;
  onEnd: (session: ExamSessionSummary) => void;
};

// One entry in the Exam Sessions list: the exam's metadata plus when the
// session was started, with Resume and End Exam actions. End Exam is reported
// via onEnd so the parent can ask for confirmation first; Resume navigates to
// the exam session page.
export default function ExamSessionCard({
  session,
  onEnd,
}: ExamSessionCardProps) {
  const router = useRouter();
  const excerpt = session.exam_excerpt;

  // The exam session page fetches the session by id (including its current
  // question index and exam metadata) from the server, so the Resume link only
  // needs to carry the session id.
  const resumeParams = new URLSearchParams({
    exam_session_id: session.exam_session_id,
  });
  return (
    <ListItem disableGutters sx={{ mb: 1 }}>
      <Card sx={{ width: "100%" }}>
        <CardContent>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <Box sx={{ flexGrow: 1, minWidth: 0 }}>
              <Typography variant="h6" component="div" noWrap>
                {excerpt.Title}
              </Typography>
              <ExamMetadataChips exam={excerpt} />
              <Typography variant="body2" color="textSecondary">
                Started{" "}
                <Tooltip title={new Date(session.started_at).toLocaleString()}>
                  <Box component="span">
                    {formatDistanceToNow(new Date(session.started_at), {
                      addSuffix: true,
                    })}
                  </Box>
                </Tooltip>
              </Typography>
            </Box>
            <Button
              variant="contained"
              sx={{ whiteSpace: "nowrap" }}
              onClick={() => router.push(`/examsession?${resumeParams}`)}
            >
              Resume
            </Button>
            <Button
              variant="contained"
              color="error"
              sx={{ whiteSpace: "nowrap" }}
              onClick={() => onEnd(session)}
            >
              End Exam
            </Button>
          </Box>
        </CardContent>
      </Card>
    </ListItem>
  );
}
