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
import { useExamDocs } from "@/hooks/useExamDocs";
import { useExamSessions } from "@/hooks/useExamSessions";
import { useEndExamSession } from "@/hooks/useEndExamSession";
import ExamCard from "@/components/ExamCard";
import ExamOptionsDialog from "@/components/ExamOptionsDialog";
import ExamSessionCard from "@/components/ExamSessionCard";
import { ExamExcerpt, ExamSessionSummary } from "@/api/types";

export default function Home() {
  const { data: exams, isPending: isExamsPending } = useExamDocs();
  const { data: sessions, isPending: isSessionsPending } = useExamSessions();
  const endSession = useEndExamSession();
  const [sessionToEnd, setSessionToEnd] = useState<ExamSessionSummary | null>(
    null,
  );
  const [examToTake, setExamToTake] = useState<ExamExcerpt | null>(null);

  return (
    <Box>
      <Box sx={{ mt: 4 }}>
        <Typography variant="h4" component="h2" gutterBottom>
          Exam Sessions
        </Typography>
        <Typography gutterBottom>
          {!isSessionsPending && sessions.length === 0
            ? "No ongoing exam sessions."
            : "Here are the ongoing exam sessions"}
        </Typography>
        {isSessionsPending ? (
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
      </Box>

      <ExamOptionsDialog
        exam={examToTake}
        onClose={() => setExamToTake(null)}
      />

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
      <Box sx={{ mt: 4 }}>
        <Typography variant="h4" component="h2" gutterBottom>
          Exams
        </Typography>
        <Typography gutterBottom>
          {!isExamsPending && exams.length === 0
            ? "No exam is found"
            : "Here are the exams you can take"}
        </Typography>
        {isExamsPending ? (
          <Typography>…</Typography>
        ) : (
          exams.length > 0 && (
            <List>
              {exams.map((exam) => (
                <ExamCard key={exam.Id} exam={exam} onTake={setExamToTake} />
              ))}
            </List>
          )
        )}
      </Box>
    </Box>
  );
}
