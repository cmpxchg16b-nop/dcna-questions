"use client";

import { useRef, useState } from "react";
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  LinearProgress,
  Typography,
} from "@mui/material";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import ErrorIcon from "@mui/icons-material/Error";
import VerifiedUserIcon from "@mui/icons-material/VerifiedUser";
import { useTranslation } from "react-i18next";
import { useVerifyCert } from "@/hooks/useVerifyCert";
import { UploadAbortedError } from "@/hooks/useUploadFile";
import { CertVerificationResponse } from "@/api/types";
import { localeTagFor } from "@/i18n";

// VerifyState is the dialog's state machine: a file selection starts an
// "uploading" phase (determinate progress bar fed by xhr.upload.onprogress);
// when the body is fully sent the phase becomes "verifying" (indeterminate
// spinner) until the API response arrives; the terminal phase is either
// "done" (the API answered, valid or not) or "failed" (the request itself
// failed — network error or a 400 on a malformed payload).
type VerifyState =
  | { phase: "uploading"; filename: string; progress: number | null }
  | { phase: "verifying"; filename: string }
  | { phase: "done"; filename: string; response: CertVerificationResponse }
  | { phase: "failed"; filename: string; message: string };

// InfoRow renders one label/value pair of the verification result: the exam
// report's and the signing certificate's fields.
function InfoRow({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <Box sx={{ display: "flex", gap: 2, py: 0.25 }}>
      <Typography
        variant="body2"
        color="textSecondary"
        sx={{ minWidth: 150, flexShrink: 0 }}
      >
        {label}
      </Typography>
      <Typography
        variant="body2"
        sx={
          mono
            ? { fontFamily: "monospace", wordBreak: "break-all" }
            : { wordBreak: "break-word" }
        }
      >
        {value}
      </Typography>
    </Box>
  );
}

// The Verify Cert button of the Trackings section plus its verification
// dialog. The hidden file input accepts a single .xml file — the signed exam
// report the exam taker received by email — and the dialog follows the
// request through upload, verification, and the resulting status: VALID with
// the verified exam report (exam taker, title, overall result, score, finish
// time) and the signing certificate, INVALID with the reason, or the request
// error. Closing the dialog mid-flight aborts the request.
export default function VerifyCertButton() {
  const { t, i18n } = useTranslation();
  const verifyCert = useVerifyCert();
  const [state, setState] = useState<VerifyState | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  const busy = state?.phase === "uploading" || state?.phase === "verifying";

  // close dismisses the dialog; an in-flight request is aborted, which lands
  // as an UploadAbortedError in the catch below and is ignored there.
  const close = () => {
    controllerRef.current?.abort();
    controllerRef.current = null;
    setState(null);
  };

  const handleFileSelected = (file: File) => {
    const controller = new AbortController();
    controllerRef.current = controller;
    setState({ phase: "uploading", filename: file.name, progress: null });
    void verifyCert
      .mutateAsync({
        file,
        signal: controller.signal,
        onProgress: (percent) =>
          setState((s) => {
            if (!s || (s.phase !== "uploading" && s.phase !== "verifying")) {
              return s;
            }
            // A fully sent body means the upload is over and the server is
            // verifying; the response is still pending.
            return percent >= 100
              ? { phase: "verifying", filename: s.filename }
              : { phase: "uploading", filename: s.filename, progress: percent };
          }),
      })
      .then((response) =>
        setState((s) =>
          s ? { phase: "done", filename: s.filename, response } : s,
        ),
      )
      .catch((error: unknown) => {
        // A deliberate close already reset the state; nothing to show.
        if (error instanceof UploadAbortedError) return;
        const message = error instanceof Error ? error.message : String(error);
        setState((s) =>
          s ? { phase: "failed", filename: s.filename, message } : s,
        );
      });
  };

  const response = state?.phase === "done" ? state.response : null;
  const report = response?.report;
  const certificate = response?.certificate;
  const examTaker = report?.examTaker.persons?.[0]?.name;
  const score = report?.assessment.scoreResult;
  const passed = report?.assessment.overallResult === "pass";
  const localeTag = localeTagFor(i18n.language);

  return (
    <>
      {/* The hidden file input lives inside the button's label so clicking
          the button opens the system file picker; resetting target.value lets
          the same file be picked again in a row. Only one .xml file at a
          time: no multiple attribute, and the button is disabled while a
          verification is in flight. */}
      <Button
        component="label"
        variant="outlined"
        startIcon={<VerifiedUserIcon />}
        disabled={busy}
        sx={{ whiteSpace: "nowrap" }}
      >
        {t("verify.button")}
        <input
          type="file"
          hidden
          accept=".xml,application/xml,text/xml"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) handleFileSelected(file);
            e.target.value = "";
          }}
        />
      </Button>

      <Dialog open={state !== null} onClose={close} fullWidth maxWidth="sm">
        <DialogTitle>{t("verify.dialogTitle")}</DialogTitle>
        <DialogContent>
          {state?.phase === "uploading" && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <LinearProgress
                variant={
                  state.progress === null ? "indeterminate" : "determinate"
                }
                value={state.progress ?? 0}
                sx={{ flexGrow: 1 }}
              />
              <Typography
                variant="body2"
                color="textSecondary"
                sx={{ whiteSpace: "nowrap" }}
              >
                {state.progress === null
                  ? t("verify.uploading")
                  : `${state.progress}%`}
              </Typography>
            </Box>
          )}
          {state?.phase === "verifying" && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              <CircularProgress size={24} />
              <Typography>{t("verify.verifying")}</Typography>
            </Box>
          )}
          {response &&
            (response.valid ? (
              <Box>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <CheckCircleIcon color="success" />
                  <Typography
                    variant="h6"
                    component="div"
                    color="success.main"
                  >
                    {t("verify.statusValid")}
                  </Typography>
                </Box>
                {report && (
                  <Box sx={{ mt: 2 }}>
                    {examTaker && (
                      <InfoRow label={t("verify.examTaker")} value={examTaker} />
                    )}
                    {report.title && (
                      <InfoRow
                        label={t("verify.examTitle")}
                        value={report.title}
                      />
                    )}
                    {report.examCode && (
                      <InfoRow
                        label={t("verify.examCode")}
                        value={report.examCode}
                      />
                    )}
                    {report.assessment.overallResult && (
                      <InfoRow
                        label={t("verify.overallResult")}
                        value={
                          passed ? t("results.pass") : t("results.fail")
                        }
                      />
                    )}
                    {score && (
                      <InfoRow
                        label={t("verify.score")}
                        value={`${score.earnedScore}/${score.totalScore}`}
                      />
                    )}
                    <InfoRow
                      label={t("verify.finishedAt")}
                      value={new Date(report.finishedAt).toLocaleString(
                        localeTag,
                      )}
                    />
                  </Box>
                )}
                {certificate && (
                  <Box sx={{ mt: 2 }}>
                    <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
                      {t("verify.certTitle")}
                    </Typography>
                    <InfoRow
                      label={t("verify.subject")}
                      value={certificate.subject}
                    />
                    <InfoRow
                      label={t("verify.issuer")}
                      value={certificate.issuer}
                    />
                    <InfoRow
                      label={t("verify.serialNumber")}
                      value={certificate.serial_number}
                    />
                    <InfoRow
                      label={t("verify.notBefore")}
                      value={new Date(certificate.not_before).toLocaleString(
                        localeTag,
                      )}
                    />
                    <InfoRow
                      label={t("verify.notAfter")}
                      value={new Date(certificate.not_after).toLocaleString(
                        localeTag,
                      )}
                    />
                    <InfoRow
                      label={t("verify.fingerprint")}
                      value={certificate.sha256_fingerprint}
                      mono
                    />
                  </Box>
                )}
              </Box>
            ) : (
              <Box>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <ErrorIcon color="error" />
                  <Typography variant="h6" component="div" color="error.main">
                    {t("verify.statusInvalid")}
                  </Typography>
                </Box>
                {response.error && (
                  <DialogContentText sx={{ mt: 1 }}>
                    {response.error}
                  </DialogContentText>
                )}
              </Box>
            ))}
          {state?.phase === "failed" && (
            <Box>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <ErrorIcon color="error" />
                <Typography variant="h6" component="div" color="error.main">
                  {t("verify.requestFailed")}
                </Typography>
              </Box>
              <DialogContentText sx={{ mt: 1 }}>
                {state.message}
              </DialogContentText>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={close}>{t("common.close")}</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
