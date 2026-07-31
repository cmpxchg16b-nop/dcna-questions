// Mock exam session data. No API calls yet — this stands in for a future
// /api/examsessions endpoint.

export type ExamSessionExcerpt = {
  Id: string;
  StartedAt: string; // ISO 8601 timestamp
  ExamTitle: string;
  ExamCode: string;
  ExamShortName: string;
};

export const mockExamSessions: ExamSessionExcerpt[] = [
  {
    Id: "sess-01JZK4Q8F3A2M9X7T5R1",
    StartedAt: "2026-07-30T14:22:00Z",
    ExamTitle: "Data Center Network Associate Practice Exam",
    ExamCode: "DCNA-100",
    ExamShortName: "dcna-practice",
  },
  {
    Id: "sess-01JZK5B1N8C4P6W2V9Q3",
    StartedAt: "2026-07-28T09:05:00Z",
    ExamTitle: "VXLAN and EVPN Fundamentals Quiz",
    ExamCode: "DCNA-210",
    ExamShortName: "vxlan-evpn-quiz",
  },
  {
    Id: "sess-01JZK6D7H2E8R4Y6U1S5",
    StartedAt: "2026-07-25T18:47:00Z",
    ExamTitle: "Data Center Network Associate Practice Exam",
    ExamCode: "DCNA-100",
    ExamShortName: "dcna-practice",
  },
];
