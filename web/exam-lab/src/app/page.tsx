"use client";

import { useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
  List,
  ListItem,
  Tooltip,
  Typography,
} from "@mui/material";
import { formatDistanceToNow } from "date-fns";
import { useExamDocs } from "@/hooks/useExamDocs";
import { useExamSessions } from "@/hooks/useExamSessions";
import { useCreateExamSession } from "@/hooks/useCreateExamSession";
import { useEndExamSession } from "@/hooks/useEndExamSession";
import {
  ExamExcerpt,
  ExamOptionRandomOptions,
  ExamOptionRandomQuestions,
  ExamSessionSummary,
} from "@/api/types";

export default function Home() {
  const { data: exams, isPending: isExamsPending } = useExamDocs();
  const { data: sessions, isPending: isSessionsPending } = useExamSessions();
  const createSession = useCreateExamSession();
  const endSession = useEndExamSession();
  const [sessionToEnd, setSessionToEnd] = useState<ExamSessionSummary | null>(
    null,
  );
  const [examToTake, setExamToTake] = useState<ExamExcerpt | null>(null);
  const [randomQuestions, setRandomQuestions] = useState(false);
  const [randomOptions, setRandomOptions] = useState(false);

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
                <ListItem
                  key={session.exam_session_id}
                  disableGutters
                  sx={{ mb: 1 }}
                >
                  <Card sx={{ width: "100%" }}>
                    <CardContent>
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 2 }}
                      >
                        <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                          <Typography variant="h6" component="div" noWrap>
                            {session.exam_excerpt.Title}
                          </Typography>
                          <Typography variant="body2" color="text.secondary">
                            {session.exam_excerpt.ShortName} ·{" "}
                            {session.exam_excerpt.Code}
                          </Typography>
                          <Typography variant="body2" color="text.secondary">
                            Started{" "}
                            <Tooltip
                              title={new Date(
                                session.started_at,
                              ).toLocaleString()}
                            >
                              <Box component="span">
                                {formatDistanceToNow(
                                  new Date(session.started_at),
                                  {
                                    addSuffix: true,
                                  },
                                )}
                              </Box>
                            </Tooltip>
                          </Typography>
                        </Box>
                        <Button
                          variant="contained"
                          sx={{ whiteSpace: "nowrap" }}
                          onClick={() => alert("unimplemented")}
                        >
                          Resume
                        </Button>
                        <Button
                          variant="contained"
                          color="error"
                          sx={{ whiteSpace: "nowrap" }}
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
          )
        )}
      </Box>

      <Dialog open={examToTake !== null} onClose={() => setExamToTake(null)}>
        <DialogTitle>{examToTake?.Title}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {examToTake?.ShortName} · {examToTake?.Code}
          </DialogContentText>
          <Box sx={{ display: "flex", flexDirection: "column" }}>
            <FormControlLabel
              control={
                <Checkbox
                  checked={randomQuestions}
                  onChange={(e) => setRandomQuestions(e.target.checked)}
                />
              }
              label="Randomized questions order"
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={randomOptions}
                  onChange={(e) => setRandomOptions(e.target.checked)}
                />
              }
              label="Randomized options order"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExamToTake(null)}>Cancel</Button>
          <Button
            variant="contained"
            loading={createSession.isPending}
            onClick={() => {
              if (!examToTake) return;
              const options =
                (randomQuestions ? ExamOptionRandomQuestions : 0) |
                (randomOptions ? ExamOptionRandomOptions : 0);
              createSession.mutate(
                { examId: examToTake.Id, options },
                { onSuccess: () => setExamToTake(null) },
              );
            }}
          >
            Take
          </Button>
        </DialogActions>
      </Dialog>

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
                <ListItem key={exam.Id} disableGutters sx={{ mb: 1 }}>
                  <Card sx={{ width: "100%" }}>
                    <CardContent>
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 2 }}
                      >
                        {/* minWidth: 0 lets the text column shrink so the clamp can
                          kick in instead of pushing the button off-card. */}
                        <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                          <Typography variant="h6" component="div" noWrap>
                            {exam.Title}
                          </Typography>
                          <Typography
                            gutterBottom
                            variant="body2"
                            color="text.secondary"
                          >
                            {exam.ShortName} · {exam.Code} · {exam.NumQuestions}{" "}
                            {exam.NumQuestions === 1 ? "question" : "questions"}
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
                          onClick={() => {
                            setRandomQuestions(false);
                            setRandomOptions(false);
                            setExamToTake(exam);
                          }}
                        >
                          Take
                        </Button>
                      </Box>
                    </CardContent>
                  </Card>
                </ListItem>
              ))}
            </List>
          )
        )}
      </Box>
    </Box>
  );
}
