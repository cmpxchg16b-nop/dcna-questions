"use client";

import { useState } from "react";
import { Box, Paper, Stack, Typography } from "@mui/material";
import {
  Assessment,
  Connect,
  ImgCandidate,
  ImgDrop,
  Question,
} from "@/api/types";
import { isConnectionAccepted } from "@/api/dragAndDrop";

type ImgDragAndDropBoardProps = {
  question: Question;
  // connections holds the placed candidate→drop connections. The board is
  // controlled so the page can drive the Check/Next/Skip footer state,
  // restore previously submitted placements, and reset per question — the
  // same role `selected` plays for the choice types. src is a candidate
  // nodeId, dst is a drop nodeId. The board maintains two invariants: a
  // candidate occupies at most one drop target, and a drop target holds at
  // most one candidate (placing onto an occupied target replaces its content).
  connections: Connect[];
  onConnectionsChange: (connections: Connect[]) => void;
  // disabled freezes the board while the saved answer is loading, while a
  // submission is in flight, and once the assessment is on screen.
  disabled?: boolean;
  // assessment is the practice-exam "Check" result for this question; when it
  // carries this question's connection solutions, the board marks each placed
  // connection and reveals the correct answer below the drop area.
  assessment?: Assessment | null;
};

// ImgDragAndDropBoard renders an image-based drag-and-drop question: a pool of
// candidate image snippets beside a background image that hosts absolutely
// positioned drop targets. Snippets can be dragged onto targets (HTML5 drag
// and drop) or placed by click — click a snippet to pick it up, then click a
// target to place it — which also covers touch and keyboard interaction
// (snippets and targets are focusable and respond to Enter/Space). Dragging a
// placed snippet back onto the pool, or clicking an occupied target with
// nothing picked, removes the placement.
export default function ImgDragAndDropBoard({
  question,
  connections,
  onConnectionsChange,
  disabled = false,
  assessment = null,
}: ImgDragAndDropBoardProps) {
  // picked is the candidate nodeId grabbed by click (as opposed to drag); it is
  // placed onto the next drop target the user clicks.
  const [picked, setPicked] = useState<string | null>(null);
  // dragOverDropId highlights the target currently hovered during a drag.
  const [dragOverDropId, setDragOverDropId] = useState<string | null>(null);

  const imgDragAndDrop = question.imgDragAndDrop;
  const candidates = imgDragAndDrop?.imgCandidates ?? [];
  const dropsArea = imgDragAndDrop?.imgDropsArea;
  const imgDrops = dropsArea?.imgDrops ?? [];

  const candidateById = new Map(candidates.map((c) => [c.nodeId, c]));
  const dropById = new Map(imgDrops.map((d) => [d.nodeId, d]));
  const placedByDrop = new Map(connections.map((c) => [c.dst, c.src]));
  const placedCandidates = new Set(connections.map((c) => c.src));

  // The practice-exam review data: this question's origin document inside the
  // assessment carries the connection solutions that drive the verdict markers
  // and the correct-answer reveal.
  const solutions =
    assessment?.questions?.find((q) => q.id === question.id)?.correctAnswer
      ?.connectionSolutions ?? [];

  // place drops candidateId onto dropId, preserving the invariants: the
  // candidate leaves whichever target it occupied, and the target's previous
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

  if (!imgDragAndDrop || !dropsArea) return null;

  // The correct-answer reveal speaks in labels, not nodeIds; the raw id is
  // the fallback for a solution endpoint absent from the question payload.
  const candidateLabel = (id: string) => candidateById.get(id)?.nodeLabel ?? id;
  const dropLabel = (id: string) => dropById.get(id)?.nodeLabel ?? id;

  const renderCandidate = (candidate: ImgCandidate) => {
    const id = candidate.nodeId;
    const used = placedCandidates.has(id);
    const isPicked = picked === id;
    return (
      <Box
        key={id}
        component="img"
        src={`/${candidate.imgDataSrc}`}
        alt={`Image snippet ${id}`}
        role={disabled ? undefined : "button"}
        tabIndex={disabled ? undefined : 0}
        aria-pressed={isPicked}
        draggable={!disabled}
        onDragStart={(e) => startDrag(e, id)}
        onClick={() => !disabled && setPicked(isPicked ? null : id)}
        onKeyDown={(e) => {
          if (disabled) return;
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setPicked(isPicked ? null : id);
          }
        }}
        sx={{
          width: candidate.width,
          height: candidate.height,
          objectFit: "contain",
          display: "block",
          boxSizing: "border-box",
          border: "2px solid",
          borderColor: isPicked ? "primary.main" : "divider",
          borderRadius: 1,
          cursor: disabled ? "default" : "grab",
          // Placed candidates stay visible in the pool (dimmed) so they can be
          // re-picked or re-dragged to move them between targets.
          opacity: used ? 0.45 : 1,
        }}
      />
    );
  };

  const renderDrop = (drop: ImgDrop) => {
    const candidateId = placedByDrop.get(drop.nodeId);
    const candidate = candidateId ? candidateById.get(candidateId) : undefined;
    // Verdict markers appear only once the assessment revealed this question's
    // connection solutions.
    const accepted =
      assessment && candidateId && solutions.length > 0
        ? isConnectionAccepted(
            { src: candidateId, dst: drop.nodeId },
            solutions,
          )
        : undefined;
    const hovered = dragOverDropId === drop.nodeId;
    // The drop targets overlay the background image, which is a light diagram
    // regardless of the app's color mode, so their idle styling uses fixed
    // dark tones rather than theme palette colors (theme text/divider colors
    // go light in dark mode and would vanish against the light image). Empty
    // targets get a faint dark scrim plus a clearly visible dashed border; a
    // soft light halo behind the border keeps it readable on mid-tone images.
    const idleBorder = candidate
      ? "rgba(0, 0, 0, 0.35)"
      : "rgba(0, 0, 0, 0.65)";
    return (
      <Box
        key={drop.nodeId}
        role={disabled ? undefined : "button"}
        tabIndex={disabled ? undefined : 0}
        aria-label={`Drop target ${drop.nodeId}`}
        onClick={() => onSlotClick(drop.nodeId)}
        onKeyDown={(e) => onSlotKeyDown(e, drop.nodeId)}
        onDragOver={(e) => {
          if (disabled) return;
          e.preventDefault();
          e.dataTransfer.dropEffect = "move";
          setDragOverDropId(drop.nodeId);
        }}
        onDragLeave={() => setDragOverDropId(null)}
        onDrop={(e) => onSlotDrop(e, drop.nodeId)}
        sx={{
          position: "absolute",
          left: drop.positionX,
          top: drop.positionY,
          width: drop.width,
          height: drop.height,
          boxSizing: "border-box",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          border: "2px dashed",
          borderColor:
            accepted === true
              ? "success.main"
              : accepted === false
                ? "error.main"
                : hovered
                  ? "primary.main"
                  : picked
                    ? "primary.light"
                    : idleBorder,
          // The halo is a light ring just outside the dashed border; it
          // separates the dark border from whatever the image shows beneath.
          boxShadow:
            accepted !== undefined || hovered || picked || candidate
              ? "none"
              : "0 0 0 2px rgba(255, 255, 255, 0.6)",
          borderRadius: 1,
          cursor: disabled ? "default" : "pointer",
          bgcolor: hovered
            ? "action.hover"
            : candidate
              ? "transparent"
              : "rgba(0, 0, 0, 0.08)",
        }}
      >
        {candidate && (
          <Box
            component="img"
            src={`/${candidate.imgDataSrc}`}
            alt={`Placed snippet ${candidate.nodeId}`}
            draggable={!disabled}
            onDragStart={(e) => startDrag(e, candidate.nodeId)}
            sx={{
              maxWidth: "100%",
              maxHeight: "100%",
              objectFit: "contain",
              cursor: disabled ? "default" : "grab",
            }}
          />
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
          sx={{ p: 1.5 }}
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
          <Typography variant="subtitle2" gutterBottom>
            Image snippets
          </Typography>
          <Stack spacing={1}>{candidates.map(renderCandidate)}</Stack>
        </Paper>
        <Box
          sx={{
            position: "relative",
            width: dropsArea.width,
            height: dropsArea.height,
            flexShrink: 0,
          }}
        >
          <Box
            component="img"
            src={`/${dropsArea.imgBackgroundUrl}`}
            alt="Drop area background"
            draggable={false}
            sx={{
              position: "absolute",
              inset: 0,
              width: "100%",
              height: "100%",
            }}
          />
          {imgDrops.map(renderDrop)}
        </Box>
      </Stack>

      {assessment && solutions.length > 0 && (
        <Box sx={{ mt: 2 }}>
          <Typography variant="subtitle2" gutterBottom>
            Correct answer
          </Typography>
          {solutions.map((solution, i) => (
            <Box key={i} sx={{ mb: 1 }}>
              {(solutions.length > 1 ||
                (solution.connectCombinations?.length ?? 0) > 0) && (
                <Typography variant="caption" color="textSecondary">
                  {solutions.length > 1 ? `Solution ${i + 1}: r` : "R"}
                  equires {solution.requiredUniqueConnections} unique
                  connections
                </Typography>
              )}
              {solution.connects?.map((c, j) => {
                const src = candidateById.get(c.src);
                return (
                  <Stack
                    key={`c-${j}`}
                    direction="row"
                    spacing={1}
                    sx={{ alignItems: "center" }}
                  >
                    {src && (
                      <Box
                        component="img"
                        src={`/${src.imgDataSrc}`}
                        alt={c.src}
                        sx={{
                          width: 32,
                          height: 24,
                          objectFit: "contain",
                          border: "1px solid",
                          borderColor: "divider",
                          borderRadius: 0.5,
                        }}
                      />
                    )}
                    <Typography variant="body2">
                      {candidateLabel(c.src)} → {dropLabel(c.dst)}
                    </Typography>
                  </Stack>
                );
              })}
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
                    (any pairing)
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
