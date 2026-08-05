import { useMutation, useQueryClient } from "@tanstack/react-query";

// createExamAssociation calls POST /api/examassociations with a JSON body
// carrying the upload_id, binding the upload's exam documents to the caller.
// The server responds 201 Created (400 when the upload cannot be associated,
// e.g. it is not a .tar archive; 404 when the upload does not exist). The
// caller's session_id cookie is sent automatically (same-origin fetch).
async function createExamAssociation(uploadId: string): Promise<void> {
  const res = await fetch("/api/examassociations", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ upload_id: uploadId }),
  });
  if (!res.ok)
    throw new Error(`failed to create exam association: ${res.status}`);
}

// useCreateExamAssociation creates an association for an upload. On success it
// invalidates the "examassociations" query (the key used by
// useExamAssociations) so the association list refetches with the new one.
export function useCreateExamAssociation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createExamAssociation,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["examassociations"] });
    },
  });
}
