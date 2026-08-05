import { useMutation, useQueryClient } from "@tanstack/react-query";

// deleteUpload calls DELETE /api/useruploads/{upload_id}, which removes the
// upload and responds 204 No Content (404 when the upload is already gone).
// The caller's session_id cookie is sent automatically (same-origin fetch).
async function deleteUpload(uploadId: string): Promise<void> {
  const res = await fetch(`/api/useruploads/${encodeURIComponent(uploadId)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(`failed to delete upload: ${res.status}`);
}

// useDeleteUpload removes an upload by id. On success it invalidates the
// "useruploads" query (the key used by useUploads) so the uploads list
// refetches without the deleted upload.
export function useDeleteUpload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteUpload,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["useruploads"] });
    },
  });
}
