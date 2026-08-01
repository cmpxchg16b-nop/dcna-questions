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

  const resumeParams = new URLSearchParams({
    exam_session_id: session.exam_session_id,
    title: excerpt.Title,
    shortname: excerpt.ShortName,
    code: excerpt.Code,
    num_questions: String(excerpt.NumQuestions),
    // Mocked until the session API reports real progress.
    current_question_index: "0",
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
              <Typography variant="body2" color="text.secondary">
                {excerpt.ShortName} · {excerpt.Code}
              </Typography>
              <Typography variant="body2" color="text.secondary">
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
