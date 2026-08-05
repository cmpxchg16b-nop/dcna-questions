"use client";

import { useRef, useState } from "react";
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
  IconButton,
  LinearProgress,
  List,
  ListItem,
  Typography,
} from "@mui/material";
import CancelIcon from "@mui/icons-material/Cancel";
import CloseIcon from "@mui/icons-material/Close";
import CloudUploadIcon from "@mui/icons-material/CloudUpload";
import { useUploads } from "@/hooks/useUploads";
import { UploadAbortedError, useUploadFile } from "@/hooks/useUploadFile";
import { useDeleteUpload } from "@/hooks/useDeleteUpload";
import { useExamAssociations } from "@/hooks/useExamAssociations";
import { useCreateExamAssociation } from "@/hooks/useCreateExamAssociation";
import { useDeleteExamAssociation } from "@/hooks/useDeleteExamAssociation";
import UploadCard from "@/components/UploadCard";
import { UserUploadSummary } from "@/api/types";

type UserUploadsListDisplayProps = {
  generation: number;
  onGenerationChange: () => void;
};

type PendingUpload = {
  id: number;
  file: File;
  // controller aborts the in-flight request when the caller cancels.
  controller: AbortController;
  // progress is the last reported 0–100 upload percentage, or null before
  // the first progress event arrives (rendered as an indeterminate bar).
  progress: number | null;
  // error is set when the upload request rejects; the entry then stays on
  // screen as a failed card until the caller dismisses it.
  error: string | null;
};

// Placeholder card for an in-flight (or failed) upload, rendered in the list
// alongside the uploaded files until the upload succeeds and the refetched
// uploads list replaces it with the real UploadCard. While the file streams
// out it shows a determinate progress bar fed by xhr.upload.onprogress; at
// 100% the body is fully sent but the server is still storing the file, so
// the bar switches to an indeterminate "Processing…" state until the
// response arrives. The Cancel button only reports the intent through
// onCancel; the parent asks for confirmation before actually aborting.
function PendingUploadCard({
  pending,
  onDismiss,
  onCancel,
}: {
  pending: PendingUpload;
  onDismiss: (id: number) => void;
  onCancel: (id: number) => void;
}) {
  const sent = pending.progress !== null && pending.progress >= 100;
  return (
    <ListItem disableGutters sx={{ mb: 1 }}>
      <Card sx={{ width: "100%" }}>
        <CardContent>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <Box sx={{ flexGrow: 1, minWidth: 0 }}>
              <Typography variant="h6" component="div" noWrap>
                {pending.file.name}
              </Typography>
              {pending.error ? (
                <Typography variant="body2" color="error">
                  Upload failed: {pending.error}
                </Typography>
              ) : (
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    mt: 0.5,
                  }}
                >
                  <LinearProgress
                    variant={
                      pending.progress === null || sent
                        ? "indeterminate"
                        : "determinate"
                    }
                    value={pending.progress ?? 0}
                    sx={{ flexGrow: 1 }}
                  />
                  <Typography
                    variant="body2"
                    color="textSecondary"
                    sx={{ whiteSpace: "nowrap" }}
                  >
                    {pending.progress === null
                      ? "Uploading…"
                      : sent
                        ? "Processing…"
                        : `${pending.progress}%`}
                  </Typography>
                </Box>
              )}
            </Box>
            {pending.error ? (
              <IconButton
                aria-label="Dismiss"
                onClick={() => onDismiss(pending.id)}
              >
                <CloseIcon />
              </IconButton>
            ) : (
              <IconButton
                aria-label="Cancel upload"
                onClick={() => onCancel(pending.id)}
              >
                <CancelIcon />
              </IconButton>
            )}
          </Box>
        </CardContent>
      </Card>
    </ListItem>
  );
}

// The Uploads section: the upload button, the caller's uploads (newest first),
// and the delete confirmation dialog. Each .tar card carries an Associate
// checkbox that binds the upload's exam documents to the caller via
// /api/examassociations; because that changes the set of exams the server
// serves, a successful toggle reports through onGenerationChange so the other
// sections — notably the exams list — refetch.
export default function UserUploadsListDisplay({
  generation,
  onGenerationChange,
}: UserUploadsListDisplayProps) {
  const { data: uploads, isPending } = useUploads(generation);
  const uploadFile = useUploadFile();
  const deleteUpload = useDeleteUpload();
  const { data: examAssociations } = useExamAssociations();
  const createExamAssociation = useCreateExamAssociation();
  const deleteExamAssociation = useDeleteExamAssociation();
  const [uploadToDelete, setUploadToDelete] =
    useState<UserUploadSummary | null>(null);

  // pendingUploads tracks every in-flight upload individually. The picker
  // accepts multiple files and each spawns its own upload; those run
  // concurrently (the server API is concurrency-safe), but useMutation's
  // exposed state (isPending etc.) only ever reflects the latest call, so
  // each upload gets a local entry instead — added before the request
  // starts, updated by the onProgress callback as the file streams out,
  // removed on success, marked with the error on failure.
  const [pendingUploads, setPendingUploads] = useState<PendingUpload[]>([]);
  const nextPendingId = useRef(0);

  const handleFilesSelected = (files: File[]) => {
    for (const file of files) {
      const id = nextPendingId.current++;
      const controller = new AbortController();
      setPendingUploads((prev) => [
        ...prev,
        { id, file, controller, progress: null, error: null },
      ]);
      // mutateAsync, not mutate: callbacks passed to a mutate call only
      // fire for the hook's LATEST invocation (each mutate call detaches the
      // observer from the previous mutation), so with several uploads in
      // flight the earlier ones would silently lose their onSuccess/onError
      // and their cards would hang at "Processing…" forever. The promise
      // returned by mutateAsync settles with each upload individually.
      void uploadFile
        .mutateAsync({
          file,
          signal: controller.signal,
          onProgress: (percent) =>
            setPendingUploads((prev) =>
              prev.map((p) => (p.id === id ? { ...p, progress: percent } : p)),
            ),
        })
        .then(() =>
          setPendingUploads((prev) => prev.filter((p) => p.id !== id)),
        )
        .catch((error: unknown) => {
          // A deliberate cancel just drops the card; a genuine failure keeps
          // it on screen with the error message.
          if (error instanceof UploadAbortedError) {
            setPendingUploads((prev) => prev.filter((p) => p.id !== id));
            return;
          }
          const message =
            error instanceof Error ? error.message : String(error);
          setPendingUploads((prev) =>
            prev.map((p) => (p.id === id ? { ...p, error: message } : p)),
          );
        });
    }
  };

  const dismissPendingUpload = (id: number) =>
    setPendingUploads((prev) => prev.filter((p) => p.id !== id));

  // Clicking Cancel on an uploading card asks for confirmation first — an
  // accidental tap (e.g. on a touch screen) would otherwise throw the
  // progress away. Only the id is kept in state and the entry is looked up
  // at render time, so if the upload settles while the dialog is open, the
  // dialog closes by itself.
  const [uploadToCancelId, setUploadToCancelId] = useState<number | null>(null);
  const uploadToCancel =
    uploadToCancelId === null
      ? null
      : (pendingUploads.find((p) => p.id === uploadToCancelId) ?? null);

  // Confirming aborts the underlying XHR; the resulting UploadAbortedError
  // lands in the catch above, which drops the entry.
  const handleCancelConfirm = () => {
    uploadToCancel?.controller.abort();
    setUploadToCancelId(null);
  };

  // The server keeps at most one association per upload, so indexing by
  // upload_id directly gives each upload's associated state and the
  // association id needed to un-associate.
  const associationByUploadId = new Map(
    examAssociations.map((a) => [a.upload_id, a]),
  );

  const handleAssociateChange = (
    upload: UserUploadSummary,
    associate: boolean,
  ) => {
    if (associate) {
      createExamAssociation.mutate(upload.upload_id, {
        onSuccess: onGenerationChange,
      });
      return;
    }
    const association = associationByUploadId.get(upload.upload_id);
    if (association) {
      deleteExamAssociation.mutate(association.id, {
        onSuccess: onGenerationChange,
      });
    }
  };

  // The server does not cascade upload deletion to associations: deleting an
  // associated upload would keep serving its exam documents from a file that
  // no longer exists, and with the card gone there would be no checkbox left
  // to un-associate it. So confirming a delete first removes the association
  // (if any), then deletes the upload; the generation bump after the
  // un-association lets the exams list refetch without those exam documents.
  const handleDeleteConfirm = () => {
    if (!uploadToDelete) return;
    const uploadId = uploadToDelete.upload_id;
    const association = associationByUploadId.get(uploadId);
    if (!association) {
      deleteUpload.mutate(uploadId, {
        onSuccess: () => setUploadToDelete(null),
      });
      return;
    }
    deleteExamAssociation.mutate(association.id, {
      onSuccess: () => {
        onGenerationChange();
        deleteUpload.mutate(uploadId, {
          onSuccess: () => setUploadToDelete(null),
        });
      },
    });
  };

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h4" component="h2" gutterBottom>
        Uploads
      </Typography>
      <Typography gutterBottom>
        {!isPending && uploads.length === 0
          ? "No files uploaded yet"
          : "Here are the files you have uploaded"}
      </Typography>
      {/* The hidden file input lives inside the button's label so clicking
          the button opens the file picker; resetting target.value lets the
          same file be picked again in a row. The button stays enabled while
          uploads are in flight so more files can be enqueued; progress is
          shown per upload in the list below. */}
      <Button
        component="label"
        variant="contained"
        startIcon={<CloudUploadIcon />}
        sx={{ mb: 1 }}
      >
        Upload
        <input
          type="file"
          hidden
          multiple
          onChange={(e) => {
            const files = Array.from(e.target.files ?? []);
            if (files.length > 0) handleFilesSelected(files);
            e.target.value = "";
          }}
        />
      </Button>
      {isPending ? (
        <Typography>…</Typography>
      ) : (
        (uploads.length > 0 || pendingUploads.length > 0) && (
          <List>
            {/* In-flight uploads sit in the same list, above the finished
                ones, each with its own progress bar. */}
            {pendingUploads.map((pending) => (
              <PendingUploadCard
                key={pending.id}
                pending={pending}
                onDismiss={dismissPendingUpload}
                onCancel={setUploadToCancelId}
              />
            ))}
            {/* Show the most recently uploaded file at the top. */}
            {[...uploads]
              .sort((a, b) => b.last_modified_at - a.last_modified_at)
              .map((upload) => (
                <UploadCard
                  key={upload.upload_id}
                  upload={upload}
                  onDelete={setUploadToDelete}
                  associated={associationByUploadId.has(upload.upload_id)}
                  associateBusy={
                    (createExamAssociation.isPending &&
                      createExamAssociation.variables === upload.upload_id) ||
                    (deleteExamAssociation.isPending &&
                      deleteExamAssociation.variables ===
                        associationByUploadId.get(upload.upload_id)?.id)
                  }
                  onAssociateChange={handleAssociateChange}
                />
              ))}
          </List>
        )
      )}

      <Dialog
        open={uploadToDelete !== null}
        onClose={() => setUploadToDelete(null)}
      >
        <DialogTitle>Delete upload?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Delete {uploadToDelete?.filename}? This cannot be undone.
          </DialogContentText>
          {uploadToDelete &&
            associationByUploadId.has(uploadToDelete.upload_id) && (
              <DialogContentText sx={{ mt: 1 }}>
                This file is associated; its association (and the exams it
                provides) will be removed first.
              </DialogContentText>
            )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setUploadToDelete(null)}>Cancel</Button>
          <Button
            color="error"
            variant="contained"
            loading={deleteUpload.isPending || deleteExamAssociation.isPending}
            onClick={handleDeleteConfirm}
          >
            Delete
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={uploadToCancel !== null}
        onClose={() => setUploadToCancelId(null)}
      >
        <DialogTitle>Cancel upload?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Cancel uploading {uploadToCancel?.file.name}? The progress so far
            will be lost.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setUploadToCancelId(null)}>
            Keep uploading
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={handleCancelConfirm}
          >
            Cancel upload
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
