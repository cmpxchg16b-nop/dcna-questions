"use client";

import { useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  List,
  ListItem,
  Typography,
} from "@mui/material";
import { useExamDocs } from "@/hooks/useExamDocs";
import { ExamSessionExcerpt, mockExamSessions } from "@/api/mockExamSessions";

export default function Home() {
  const { data: exams, isPending: isExamsPending } = useExamDocs();
  const [sessionToEnd, setSessionToEnd] = useState<ExamSessionExcerpt | null>(
    null,
  );

  return (
    <Box>
      <Box sx={{ mt: 4 }}>
        <Typography variant="h4" component="h2" gutterBottom>
          Exam Sessions
        </Typography>
        <List>
          {mockExamSessions.map((session) => (
            <ListItem key={session.Id} disableGutters sx={{ mb: 1 }}>
              <Card sx={{ width: "100%" }}>
                <CardContent>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
                    <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                      <Typography variant="h6" component="div" noWrap>
                        {session.ExamTitle}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {session.ExamShortName} · {session.ExamCode}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        Session {session.Id} · Started{" "}
                        {new Date(session.StartedAt).toLocaleString()}
                      </Typography>
                    </Box>
                    <Button
                      variant="contained"
                      onClick={() => alert("unimplemented")}
                    >
                      Resume
                    </Button>
                    <Button
                      variant="contained"
                      color="error"
                      onClick={() => setSessionToEnd(session)}
                    >
                      End Exam
                    </Button>
                  </Box>
                </CardContent>
              </Card>
            </ListItem>
          ))}
        </List>
      </Box>

      <Dialog
        open={sessionToEnd !== null}
        onClose={() => setSessionToEnd(null)}
      >
        <DialogTitle>End exam session?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            End session {sessionToEnd?.Id} for {sessionToEnd?.ExamTitle}? This
            cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSessionToEnd(null)}>Cancel</Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => {
              alert("unimplemented");
              setSessionToEnd(null);
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
        {isExamsPending ? (
          <Typography>…</Typography>
        ) : (
          <List>
            {exams.map((exam) => (
              <ListItem key={exam.Id} disableGutters sx={{ mb: 1 }}>
                <Card sx={{ width: "100%" }}>
                  <CardContent>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
                      {/* minWidth: 0 lets the text column shrink so the clamp can
                          kick in instead of pushing the button off-card. */}
                      <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                        <Typography variant="h6" component="div" noWrap>
                          {exam.Title}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          {exam.ShortName} · {exam.Code}
                        </Typography>
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
                      <Button
                        variant="contained"
                        onClick={() => alert("unimplemented")}
                      >
                        Take
                      </Button>
                    </Box>
                  </CardContent>
                </Card>
              </ListItem>
            ))}
          </List>
        )}
      </Box>
    </Box>
  );
}
