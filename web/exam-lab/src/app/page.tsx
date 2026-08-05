"use client";

import { useState } from "react";
import { Box } from "@mui/material";
import ExamSessionsListDisplay from "@/components/ExamSessionsListDisplay";
import ExamResultsListDisplay from "@/components/ExamResultsListDisplay";
import ExamDocumentsListDisplay from "@/components/ExamDocumentsListDisplay";
import UserUploadsListDisplay from "@/components/UserUploadsListDisplay";

export default function Home() {
  // generation is a refresh signal shared by the four sections below: each
  // one appends it to its React Query keys, so bumping it refetches every
  // section. A section whose mutation affects data surfaced by another
  // section reports it through onGenerationChange — e.g. toggling an
  // upload's Associate checkbox changes the exam documents the server
  // serves, which the exams list would otherwise never refetch. Mutations
  // with hook-level invalidations (end/create session, upload, delete) do
  // not need this: those invalidate their query key prefixes directly.
  const [generation, setGeneration] = useState(0);
  const bumpGeneration = () => setGeneration((g) => g + 1);

  return (
    <Box>
      <ExamSessionsListDisplay generation={generation} />
      <ExamResultsListDisplay generation={generation} />
      <ExamDocumentsListDisplay generation={generation} />
      <UserUploadsListDisplay
        generation={generation}
        onGenerationChange={bumpGeneration}
      />
    </Box>
  );
}
