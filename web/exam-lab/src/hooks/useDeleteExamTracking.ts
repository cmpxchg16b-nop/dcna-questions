import { useMutation, useQueryClient } from "@tanstack/react-query";

// deleteExamTracking calls DELETE /api/examtrackings/{exam_report_id}, which
// removes the report and responds 204 No Content (404 when the report is
// already gone). The caller's session_id cookie is sent automatically
// (same-origin fetch).
async function deleteExamTracking(examReportId: string): Promise<void> {
  const res = await fetch(
    `/api/examtrackings/${encodeURIComponent(examReportId)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw new Error(`failed to delete exam report: ${res.status}`);
}

// useDeleteExamTracking removes an exam report by id. On success it
// invalidates the "examtrackings" query (the key prefix used by
// useExamTrackings) so the trackings list refetches without the deleted
// report.
export function useDeleteExamTracking() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteExamTracking,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["examtrackings"] });
    },
  });
}
