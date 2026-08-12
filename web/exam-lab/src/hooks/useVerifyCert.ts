import { useMutation } from "@tanstack/react-query";
import { CertVerificationResponse } from "@/api/types";
import { UploadAbortedError } from "@/hooks/useUploadFile";

// verifyCert calls POST /api/certs/verify with the signed exam report carried
// in the "file" field of a multipart/form-data body, and resolves to the
// verification outcome. A failed verification is still a 200 (valid:false);
// only malformed requests (bad multipart, non-XML payload) produce a non-2xx
// status, reported by the API as {"err": "..."}.
//
// Like uploadFile (see useUploadFile) it uses XMLHttpRequest instead of fetch
// because fetch cannot report upload progress: xhr.upload.onprogress feeds
// onProgress with a 0–100 percentage that reaches 100 when the body is fully
// sent, before the server finishes verifying and responds — so 100 means
// "sent", not "done".
//
// The optional AbortSignal lets the caller cancel mid-flight: aborting
// rejects the promise with UploadAbortedError.
async function verifyCert(
  file: File,
  onProgress?: (percent: number) => void,
  signal?: AbortSignal,
): Promise<CertVerificationResponse> {
  return new Promise((resolve, reject) => {
    // A signal that was aborted before the request even started rejects
    // immediately.
    if (signal?.aborted) {
      reject(new UploadAbortedError());
      return;
    }
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/certs/verify");
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText) as CertVerificationResponse);
        return;
      }
      // The API reports a malformed request as {"err": "..."}; surface that
      // message when present.
      let message = `failed to verify exam report: ${xhr.status}`;
      try {
        const body = JSON.parse(xhr.responseText) as { err?: string };
        if (body.err) message = body.err;
      } catch {
        // Keep the generic message.
      }
      reject(new Error(message));
    };
    xhr.onerror = () =>
      reject(new Error("failed to verify exam report: network error"));
    // xhr.abort() fires the abort event (not error), so a cancellation
    // rejects through onabort rather than onerror.
    xhr.onabort = () => reject(new UploadAbortedError());
    signal?.addEventListener("abort", () => xhr.abort(), { once: true });
    const form = new FormData();
    form.append("file", file);
    xhr.send(form);
  });
}

// useVerifyCert uploads a signed exam report for verification. The caller may
// pass an onProgress callback alongside the file to receive upload
// percentages, and an AbortSignal to cancel mid-flight (see verifyCert). The
// mutation resolves to the API's verification outcome — including a
// valid:false outcome, which is a successful request, not a rejection.
export function useVerifyCert() {
  return useMutation({
    mutationFn: ({
      file,
      onProgress,
      signal,
    }: {
      file: File;
      onProgress?: (percent: number) => void;
      signal?: AbortSignal;
    }) => verifyCert(file, onProgress, signal),
  });
}
