"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Box } from "@mui/material";
import ExamSessionsListDisplay from "@/components/ExamSessionsListDisplay";
import ExamResultsListDisplay from "@/components/ExamResultsListDisplay";
import ExamDocumentsListDisplay from "@/components/ExamDocumentsListDisplay";
import UserUploadsListDisplay from "@/components/UserUploadsListDisplay";
import SponsorAcknowledgement from "@/components/SponsorAcknowledgement";
import { LabelFilter } from "@/api/types";

// useSearchParams bails out of prerendering up to the nearest Suspense
// boundary; under output:"export" the build fails without one.
export default function Home() {
  return (
    <Suspense>
      <HomeContent />
    </Suspense>
  );
}

function HomeContent() {
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

  // The Exams section is filtered by the page URL's query parameters: every
  // parameter name is a label key and its repeated values are the accepted
  // values for that key, forwarded to /api/examdocs/bylabel (OR within a key,
  // AND across keys). E.g. /?label1=a&label1=b&label2=c lists the exams whose
  // label1 is a or b and whose label2 is c.
  const searchParams = useSearchParams();
  const labelFilter: LabelFilter = {};
  for (const key of new Set(searchParams.keys())) {
    labelFilter[key] = searchParams.getAll(key);
  }

  return (
    <Box>
      <ExamSessionsListDisplay generation={generation} />
      <ExamResultsListDisplay generation={generation} />
      <ExamDocumentsListDisplay
        generation={generation}
        labelFilter={labelFilter}
      />
      <UserUploadsListDisplay
        generation={generation}
        onGenerationChange={bumpGeneration}
      />
      <SponsorAcknowledgement />
    </Box>
  );
}
