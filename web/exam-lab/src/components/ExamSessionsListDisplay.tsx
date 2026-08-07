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
import { useTranslation } from "react-i18next";

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
  const { t } = useTranslation();
  const { data: sessions, isPending } = useExamSessions(generation);
  const endSession = useEndExamSession();
  const [sessionToEnd, setSessionToEnd] = useState<ExamSessionSummary | null>(
    null,
  );

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h4" component="h2" gutterBottom>
        {t("sessions.title")}
      </Typography>
      <Typography gutterBottom>
        {!isPending && sessions.length === 0
          ? t("sessions.empty")
          : t("sessions.nonEmpty")}
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
        <DialogTitle>{t("sessions.endConfirmTitle")}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("sessions.endConfirmBody", {
              sessionId: sessionToEnd?.exam_session_id ?? "…",
              examTitle: sessionToEnd?.exam_excerpt.Title ?? "…",
            })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSessionToEnd(null)}>
            {t("common.cancel")}
          </Button>
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
            {t("sessions.endExam")}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
