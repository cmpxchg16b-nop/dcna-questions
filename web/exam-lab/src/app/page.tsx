"use client";

import {
  Box,
  Button,
  Card,
  CardContent,
  List,
  ListItem,
  Typography,
} from "@mui/material";
import { useExamDocs } from "@/hooks/useExamDocs";

export default function Home() {
  const { data: exams, isPending: isExamsPending } = useExamDocs();

  return (
    <Box>
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
