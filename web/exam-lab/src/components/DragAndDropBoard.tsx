"use client";

import { useState } from "react";
import { Box, Chip, Paper, Stack, Typography } from "@mui/material";
import type { SxProps, Theme } from "@mui/material";
import { Assessment, Connect, DropTarget, Question } from "@/api/types";
import { isConnectionAccepted } from "@/api/dragAndDrop";
import { useTranslation } from "react-i18next";

// Candidate labels can run to full sentences, so chips wrap their text onto
// multiple lines rather than stretching the board on one line (MUI chips
// ellipsize by default, which would hide answer text). Their width stays
// capped by the responsive maxWidth of the pool and drop sections below.
const candidateChipSx = {
  height: "auto",
  maxWidth: "100%",
  "& .MuiChip-label": {
    display: "block",
    whiteSpace: "normal",
    textAlign: "left",
    py: 0.75,
  },
} satisfies SxProps<Theme>;

type DragAndDropBoardProps = {
  question: Question;
  // connections holds the placed candidate→drop connections. The board is
  // controlled so the page can drive the Check/Next/Skip footer state,
  // restore previously submitted placements, and reset per question — the
  // same role `selected` plays for the choice types. The board maintains two
  // invariants: a candidate occupies at most one slot, and a slot holds at
  // most one candidate (placing onto an occupied slot replaces its content).
  connections: Connect[];
  onConnectionsChange: (connections: Connect[]) => void;
  // disabled freezes the board while the saved answer is loading, while a
  // submission is in flight, and once the assessment is on screen.
  disabled?: boolean;
  // assessment is the practice-exam "Check" result for this question; when it
  // carries this question's connection solutions, the board marks each placed
  // connection and reveals the correct answer below the drop zones.
  assessment?: Assessment | null;
};

// DropSection is the board's uniform view of the question's drop zone: a flat
// drops list becomes a single unlabeled section, while a multiAreaDrop
// contributes one labeled section per drop area.
type DropSection = {
  id: string;
  label?: string;
  drops: DropTarget[];
};

// DragAndDropBoard renders a drag-and-drop question: a pool of candidate
// chips on the left and the drop zones on the right. Candidates can be
// dragged onto slots (HTML5 drag and drop) or placed by click — click a
// candidate to pick it up, then click a slot to place it — which also covers
// touch and keyboard interaction (chips and slots are focusable and respond
// to Enter/Space). Dragging a placed chip back onto the pool, or clicking an
// occupied slot with nothing picked, removes the placement.
export default function DragAndDropBoard({
  question,
  connections,
  onConnectionsChange,
  disabled = false,
  assessment = null,
}: DragAndDropBoardProps) {
  const { t } = useTranslation();
  // picked is the candidate id grabbed by click (as opposed to drag); it is
  // placed onto the next slot the user clicks.
  const [picked, setPicked] = useState<string | null>(null);
  // dragOverDropId highlights the slot currently hovered during a drag.
  const [dragOverDropId, setDragOverDropId] = useState<string | null>(null);

  const sections: DropSection[] = question.multiAreaDrop
    ? question.multiAreaDrop.dropAreas
    : [{ id: "drops", drops: question.drops ?? [] }];

  const candidateById = new Map(
    (question.candidates ?? []).map((c) => [c.id, c]),
  );
  const placedByDrop = new Map(connections.map((c) => [c.dst, c.src]));
  const placedCandidates = new Set(connections.map((c) => c.src));
  const dropSectionByDropId = new Map<string, DropSection>();
  for (const section of sections) {
    for (const drop of section.drops) dropSectionByDropId.set(drop.id, section);
  }

  // The practice-exam review data: this question's origin document inside the
  // assessment carries the connection solutions that drive the verdict
  // markers and the correct-answer reveal. (The backend grader does not score
  // drag-and-drop yet, so solutions stay empty until it does.)
  const solutions =
    assessment?.questions?.find((q) => q.id === question.id)?.correctAnswer
      ?.connectionSolutions ?? [];

  // place drops candidateId onto dropId, preserving the invariants: the
  // candidate leaves whichever slot it occupied, and the slot's previous
  // content returns to the pool.
  const place = (candidateId: string, dropId: string) => {
    if (disabled) return;
    onConnectionsChange([
      ...connections.filter((c) => c.src !== candidateId && c.dst !== dropId),
      { src: candidateId, dst: dropId },
    ]);
    setPicked(null);
  };

  const unplaceDrop = (dropId: string) => {
    if (disabled) return;
    onConnectionsChange(connections.filter((c) => c.dst !== dropId));
  };

  const unplaceCandidate = (candidateId: string) => {
    if (disabled) return;
    onConnectionsChange(connections.filter((c) => c.src !== candidateId));
  };

  const onSlotClick = (dropId: string) => {
    if (disabled) return;
    if (picked !== null) {
      place(picked, dropId);
    } else if (placedByDrop.has(dropId)) {
      unplaceDrop(dropId);
    }
  };

  const onSlotKeyDown = (e: React.KeyboardEvent, dropId: string) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onSlotClick(dropId);
    }
  };

  const startDrag = (e: React.DragEvent, candidateId: string) => {
    e.dataTransfer.setData("text/plain", candidateId);
    e.dataTransfer.effectAllowed = "move";
  };

  const onSlotDrop = (e: React.DragEvent, dropId: string) => {
    e.preventDefault();
    setDragOverDropId(null);
    const candidateId = e.dataTransfer.getData("text/plain");
    if (candidateId && candidateById.has(candidateId)) {
      place(candidateId, dropId);
    }
  };

  const candidateLabel = (id: string) => candidateById.get(id)?.content ?? id;
  const dropLabel = (id: string) => {
    const section = dropSectionByDropId.get(id);
    const drop = section?.drops.find((d) => d.id === id);
    if (drop?.content) return drop.content;
    return section?.label
      ? t("question.labelSlot", { label: section.label, id })
      : t("question.slot", { id });
  };

  const renderCandidate = (id: string) => {
    const candidate = candidateById.get(id);
    if (!candidate) return null;
    const used = placedCandidates.has(id);
    return (
      <Chip
        key={id}
        label={candidate.content}
        variant={picked === id ? "filled" : "outlined"}
        color={picked === id ? "primary" : "default"}
        disabled={disabled}
        onClick={
          disabled ? undefined : () => setPicked(picked === id ? null : id)
        }
        draggable={!disabled}
        onDragStart={(e) => startDrag(e, id)}
        sx={{
          ...candidateChipSx,
          justifyContent: "flex-start",
          // Placed candidates stay visible in the pool (dimmed) so they can
          // be re-picked or re-dragged to move them between slots.
          opacity: used ? 0.45 : 1,
        }}
      />
    );
  };

  const renderSlot = (drop: DropTarget) => {
    const candidateId = placedByDrop.get(drop.id);
    const candidate = candidateId ? candidateById.get(candidateId) : undefined;
    // Verdict markers appear only once the assessment revealed this
    // question's connection solutions.
    const accepted =
      assessment && candidateId && solutions.length > 0
        ? isConnectionAccepted({ src: candidateId, dst: drop.id }, solutions)
        : undefined;
    return (
      <Box
        key={drop.id}
        role={disabled ? undefined : "button"}
        tabIndex={disabled ? undefined : 0}
        aria-label={drop.content || t("question.dropSlot", { id: drop.id })}
        onClick={() => onSlotClick(drop.id)}
        onKeyDown={(e) => onSlotKeyDown(e, drop.id)}
        onDragOver={(e) => {
          if (disabled) return;
          e.preventDefault();
          e.dataTransfer.dropEffect = "move";
          setDragOverDropId(drop.id);
        }}
        onDragLeave={() => setDragOverDropId(null)}
        onDrop={(e) => onSlotDrop(e, drop.id)}
        sx={{
          border: "1px dashed",
          borderColor:
            dragOverDropId === drop.id
              ? "primary.main"
              : picked
                ? "primary.light"
                : "divider",
          borderRadius: 1,
          p: 1,
          minWidth: 200,
          minHeight: 64,
          cursor: disabled ? "default" : "pointer",
          bgcolor: dragOverDropId === drop.id ? "action.hover" : "transparent",
        }}
      >
        {drop.content && (
          <Typography
            variant="caption"
            color="textSecondary"
            gutterBottom
            sx={{ display: "block" }}
          >
            {drop.content}
          </Typography>
        )}
        {candidate ? (
          <>
            <Chip
              label={candidate.content}
              color={
                accepted === undefined
                  ? "primary"
                  : accepted
                    ? "success"
                    : "error"
              }
              disabled={disabled}
              draggable={!disabled}
              onDragStart={(e) => startDrag(e, candidate.id)}
              onDelete={disabled ? undefined : () => unplaceDrop(drop.id)}
              sx={candidateChipSx}
            />
            {accepted !== undefined && (
              <Typography
                component="span"
                variant="caption"
                color={accepted ? "success" : "error"}
                sx={{ ml: 1, fontWeight: 600 }}
              >
                {accepted
                  ? t("question.yourAnswerCorrect")
                  : t("question.yourAnswerIncorrect")}
              </Typography>
            )}
          </>
        ) : (
          <Typography variant="body2" color="textSecondary">
            {picked
              ? t("question.placeHere", { label: candidateLabel(picked) })
              : t("question.dropHere")}
          </Typography>
        )}
      </Box>
    );
  };

  return (
    <Box>
      <Stack
        direction="row"
        spacing={2}
        useFlexGap
        sx={{ flexWrap: "wrap", alignItems: "flex-start" }}
      >
        <Paper
          variant="outlined"
          // The pool shares the row with the drop sections: full width on
          // narrow screens (the sections wrap below), half the row beyond
          // that so a long candidate can't squeeze the drop zones.
          sx={{ p: 1.5, minWidth: 200, maxWidth: { xs: "100%", sm: "50%" } }}
          onDragOver={(e) => {
            if (disabled) return;
            e.preventDefault();
            e.dataTransfer.dropEffect = "move";
          }}
          onDrop={(e) => {
            e.preventDefault();
            const candidateId = e.dataTransfer.getData("text/plain");
            if (candidateId) unplaceCandidate(candidateId);
          }}
        >
          <Stack spacing={1}>
            {(question.candidates ?? []).map((c) => renderCandidate(c.id))}
          </Stack>
        </Paper>
        {sections.map((section) => (
          // 45% leaves room for the 50% pool plus the row gap, so pool and
          // sections stay side by side on wide screens; a placed long
          // candidate wraps within its section instead of ballooning it.
          <Box key={section.id} sx={{ maxWidth: { xs: "100%", sm: "45%" } }}>
            {section.label && (
              <Typography variant="subtitle2" gutterBottom>
                {section.label}
              </Typography>
            )}
            <Stack spacing={1}>{section.drops.map(renderSlot)}</Stack>
          </Box>
        ))}
      </Stack>

      {assessment && solutions.length > 0 && (
        <Box sx={{ mt: 2 }}>
          <Typography variant="subtitle2" gutterBottom>
            {t("question.correctAnswer")}
          </Typography>
          {solutions.map((solution, i) => (
            <Box key={i} sx={{ mb: 1 }}>
              {(solutions.length > 1 ||
                (solution.connectCombinations?.length ?? 0) > 0) && (
                <Typography variant="caption" color="textSecondary">
                  {solutions.length > 1
                    ? t("question.solutionRequires", {
                        n: i + 1,
                        count: solution.requiredUniqueConnections,
                      })
                    : t("question.requiresConnections", {
                        count: solution.requiredUniqueConnections,
                      })}
                </Typography>
              )}
              {solution.connects?.map((c, j) => (
                <Typography key={`c-${j}`} variant="body2">
                  {candidateLabel(c.src)} → {dropLabel(c.dst)}
                </Typography>
              ))}
              {solution.connectCombinations?.map((combo, j) => (
                <Typography key={`cc-${j}`} variant="body2">
                  {combo.connectSources
                    ?.map((s) => candidateLabel(s.id))
                    .join(", ")}{" "}
                  →{" "}
                  {combo.connectDestinations
                    ?.map((d) => dropLabel(d.id))
                    .join(", ")}{" "}
                  <Typography
                    component="span"
                    variant="caption"
                    color="textSecondary"
                  >
                    {t("question.anyPairing")}
                  </Typography>
                </Typography>
              ))}
            </Box>
          ))}
        </Box>
      )}
    </Box>
  );
}
