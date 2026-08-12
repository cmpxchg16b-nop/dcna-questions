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
import { useExamTrackings } from "@/hooks/useExamTrackings";
import { useDeleteExamTracking } from "@/hooks/useDeleteExamTracking";
import ExamReportCard from "@/components/ExamReportCard";
import VerifyCertButton from "@/components/VerifyCertButton";
import { ExamReport } from "@/api/types";
import { useTranslation } from "react-i18next";

type ExamResultsListDisplayProps = {
  generation: number;
};

// The Trackings section: the caller's finished-exam reports, most recently
// finished first, plus the confirmation dialog for deleting one. Deleting a
// report touches no other section, so useDeleteExamTracking's own
// "examtrackings" invalidation is enough — no generation bump needed.
export default function ExamResultsListDisplay({
  generation,
}: ExamResultsListDisplayProps) {
  const { t } = useTranslation();
  const { data: reports, isPending } = useExamTrackings(generation);
  const deleteTracking = useDeleteExamTracking();
  const [reportToDelete, setReportToDelete] = useState<ExamReport | null>(null);

  return (
    <Box sx={{ mt: 4 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
        }}
      >
        <Typography variant="h4" component="h2" gutterBottom>
          {t("results.title")}
        </Typography>
        <VerifyCertButton />
      </Box>
      <Typography gutterBottom>
        {!isPending && reports.length === 0
          ? t("results.empty")
          : t("results.nonEmpty")}
      </Typography>
      {isPending ? (
        <Typography>…</Typography>
      ) : (
        reports.length > 0 && (
          <List>
            {/* The server returns reports oldest-first; show the most
                recently finished exam at the top. */}
            {[...reports].reverse().map((report) => (
              <ExamReportCard
                key={report.id}
                report={report}
                onDelete={setReportToDelete}
              />
            ))}
          </List>
        )
      )}

      <Dialog
        open={reportToDelete !== null}
        onClose={() => setReportToDelete(null)}
      >
        <DialogTitle>{t("results.deleteConfirmTitle")}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("results.deleteConfirmBody", {
              title: reportToDelete?.title ?? "…",
            })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setReportToDelete(null)}>
            {t("common.cancel")}
          </Button>
          <Button
            color="error"
            variant="contained"
            loading={deleteTracking.isPending}
            onClick={() => {
              if (!reportToDelete) return;
              deleteTracking.mutate(reportToDelete.id, {
                onSuccess: () => setReportToDelete(null),
              });
            }}
          >
            {t("common.delete")}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
