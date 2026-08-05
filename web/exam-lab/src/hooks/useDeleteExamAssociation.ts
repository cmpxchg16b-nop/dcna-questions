import { useMutation, useQueryClient } from "@tanstack/react-query";

// deleteExamAssociation calls DELETE /api/examassociations/{association_id},
// which removes the association and responds 204 No Content (404 when the
// association is already gone). The caller's session_id cookie is sent
// automatically (same-origin fetch).
async function deleteExamAssociation(associationId: string): Promise<void> {
  const res = await fetch(
    `/api/examassociations/${encodeURIComponent(associationId)}`,
    { method: "DELETE" },
  );
  if (!res.ok)
    throw new Error(`failed to delete exam association: ${res.status}`);
}

// useDeleteExamAssociation removes an association by id. On success it
// invalidates the "examassociations" query (the key used by
// useExamAssociations) so the association list refetches without the deleted
// one.
export function useDeleteExamAssociation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteExamAssociation,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["examassociations"] });
    },
  });
}
