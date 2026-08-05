"use client";

import { useState } from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  List,
  Typography,
} from "@mui/material";
import { useExamSessions } from "@/hooks/useExamSessions";
import { useEndExamSession } from "@/hooks/useEndExamSession";
import ExamSessionCard from "@/components/ExamSessionCard";
import { ExamSessionSummary } from "@/api/types";

type ExamSessionsListDisplayProps = {
  generation: number;
};

// The Exam Sessions section: the caller's ongoing sessions, plus the
// confirmation dialog for ending one. Ending a session also persists its exam
// report server-side; useEndExamSession already invalidates both the
// "examsessions" and "examtrackings" queries, so the results section refreshes
// without a generation bump.
export default function ExamSessionsListDisplay({
  generation,
}: ExamSessionsListDisplayProps) {
  const { data: sessions, isPending } = useExamSessions(generation);
  const endSession = useEndExamSession();
  const [sessionToEnd, setSessionToEnd] = useState<ExamSessionSummary | null>(
    null,
  );

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h4" component="h2" gutterBottom>
        Exam Sessions
      </Typography>
      <Typography gutterBottom>
        {!isPending && sessions.length === 0
          ? "No ongoing exam sessions"
          : "Here are the ongoing exam sessions"}
      </Typography>
      {isPending ? (
        <Typography>…</Typography>
      ) : (
        sessions.length > 0 && (
          <List>
            {sessions.map((session) => (
              <ExamSessionCard
                key={session.exam_session_id}
                session={session}
                onEnd={setSessionToEnd}
              />
            ))}
          </List>
        )
      )}

      <Dialog
        open={sessionToEnd !== null}
        onClose={() => setSessionToEnd(null)}
      >
        <DialogTitle>End exam session?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            End session {sessionToEnd?.exam_session_id} for{" "}
            {sessionToEnd?.exam_excerpt.Title}? This cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSessionToEnd(null)}>Cancel</Button>
          <Button
            color="error"
            variant="contained"
            loading={endSession.isPending}
            onClick={() => {
              if (!sessionToEnd) return;
              endSession.mutate(sessionToEnd.exam_session_id, {
                onSuccess: () => setSessionToEnd(null),
              });
            }}
          >
            End Exam
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
