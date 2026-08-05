import { useQuery } from "@tanstack/react-query";
import { ExamAssociationListResponse, ExamAssociations } from "@/api/types";

// fetchExamAssociations calls GET /api/examassociations, which returns the
// caller's associations as a single JSON object {"associations": [...]}. The
// session middleware identifies the caller via the session_id cookie, which
// same-origin fetch sends automatically.
async function fetchExamAssociations(): Promise<ExamAssociations> {
  const res = await fetch("/api/examassociations");
  if (!res.ok)
    throw new Error(`failed to fetch exam associations: ${res.status}`);
  const body = (await res.json()) as ExamAssociationListResponse;
  return body.associations;
}

// useExamAssociations fetches and caches the caller's exam associations under
// the "examassociations" query key. `data` is always a defined array (empty
// while the first request is pending), and `isPending` is true during the
// initial fetch so callers can show a loading placeholder.
export function useExamAssociations(): {
  data: ExamAssociations;
  isPending: boolean;
} {
  const { data = [], isPending } = useQuery({
    queryKey: ["examassociations"],
    queryFn: fetchExamAssociations,
  });
  return { data, isPending };
}
