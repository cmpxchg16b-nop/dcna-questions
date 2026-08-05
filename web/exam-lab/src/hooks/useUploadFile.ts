import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UserUploadSummary } from "@/api/types";

// UploadAbortedError rejects uploadFile's promise when the caller cancels
// through the AbortSignal, letting the UI tell a deliberate cancel apart
// from a genuine failure.
export class UploadAbortedError extends Error {
  constructor() {
    super("upload cancelled");
    this.name = "UploadAbortedError";
  }
}

// uploadFile calls POST /api/useruploads with the file carried in the "file"
// field of a multipart/form-data body, and resolves to the new upload's
// summary. The server takes the filename and MIME type from the multipart part
// headers, so nothing else needs to be sent. The caller's session_id cookie is
// sent automatically (same-origin request).
//
// It uses XMLHttpRequest instead of fetch because fetch cannot report upload
// progress; xhr.upload.onprogress fires as the request body streams out and
// feeds onProgress with a 0–100 percentage. The percentage reaches 100 when
// the body is fully sent, before the server finishes storing the file and
// responds — so 100 means "sent", not "done".
//
// The optional AbortSignal lets the caller cancel mid-flight: aborting
// rejects the promise with UploadAbortedError. Cancellation only stops the
// client — if the body was already fully sent, the server may still finish
// storing the file.
async function uploadFile(
  file: File,
  onProgress?: (percent: number) => void,
  signal?: AbortSignal,
): Promise<UserUploadSummary> {
  return new Promise((resolve, reject) => {
    // A signal that was aborted before the request even started rejects
    // immediately.
    if (signal?.aborted) {
      reject(new UploadAbortedError());
      return;
    }
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/useruploads");
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText) as UserUploadSummary);
      } else {
        reject(new Error(`failed to upload file: ${xhr.status}`));
      }
    };
    xhr.onerror = () =>
      reject(new Error("failed to upload file: network error"));
    // xhr.abort() fires the abort event (not error), so a cancellation
    // rejects through onabort rather than onerror.
    xhr.onabort = () => reject(new UploadAbortedError());
    signal?.addEventListener("abort", () => xhr.abort(), { once: true });
    const form = new FormData();
    form.append("file", file);
    xhr.send(form);
  });
}

// useUploadFile uploads a file. The caller may pass an onProgress callback
// alongside the file to receive upload percentages, and an AbortSignal to
// cancel the upload mid-flight (see uploadFile). On success it invalidates
// the "useruploads" query (the key used by useUploads) so the uploads list
// refetches with the new upload.
export function useUploadFile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      file,
      onProgress,
      signal,
    }: {
      file: File;
      onProgress?: (percent: number) => void;
      signal?: AbortSignal;
    }) => uploadFile(file, onProgress, signal),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["useruploads"] });
    },
  });
}
