"use client";

import { useState } from "react";
import {
  Avatar,
  ButtonBase,
  Divider,
  ListItemIcon,
  Menu,
  MenuItem,
  Typography,
} from "@mui/material";
import LogoutIcon from "@mui/icons-material/Logout";
import { useProfile } from "@/hooks/useProfile";
import { useLogout } from "@/hooks/useLogout";

// avatarHue hashes the subject id to a stable hue (0–359), so each user gets
// a consistent avatar color without the server assigning one.
function avatarHue(subjectId: string): number {
  let hash = 0;
  for (let i = 0; i < subjectId.length; i++) {
    hash = (hash * 31 + subjectId.charCodeAt(i)) | 0;
  }
  return ((hash % 360) + 360) % 360;
}

// ProfileMenu is the account area at the right end of the top bar: a round
// avatar with the subject's first initial, plus the subject id text when the
// viewport is sm (600px) or wider. Clicking it opens a menu showing the
// subject id and a Log Out item. It renders nothing while the profile is
// loading or when the caller is unauthenticated (GET /api/profile 401s).
export default function ProfileMenu() {
  const { data } = useProfile();
  const logout = useLogout();
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const open = anchorEl !== null;

  const subjectId = data?.subject_id;
  if (!subjectId) return null;

  const closeMenu = () => setAnchorEl(null);

  const handleLogout = () => {
    closeMenu();
    logout.mutate(undefined, {
      onSuccess: () => {
        // Full-page navigation so the query cache and all other client state
        // tied to the old session are dropped with it.
        window.location.assign("/login");
      },
    });
  };

  return (
    <>
      <ButtonBase
        aria-label={`Account: ${subjectId}`}
        aria-controls={open ? "profile-menu" : undefined}
        aria-haspopup="true"
        aria-expanded={open ? "true" : undefined}
        onClick={(e) => setAnchorEl(e.currentTarget)}
        sx={{ borderRadius: "9999px", gap: 1, py: 0.5, pr: 1, ml: 0.5 }}
      >
        <Avatar
          sx={{
            width: 32,
            height: 32,
            fontSize: 16,
            color: "#fff",
            bgcolor: `hsl(${avatarHue(subjectId)}, 65%, 45%)`,
            border: "2px solid #fff",
            // Keeps the white ring visible against the light-scheme bar.
            boxSizing: "border-box",
          }}
        >
          {subjectId.charAt(0).toUpperCase()}
        </Avatar>
        <Typography
          variant="body2"
          noWrap
          sx={{
            display: { xs: "none", sm: "block" },
            maxWidth: { sm: 140, md: 240 },
            textOverflow: "ellipsis",
            overflow: "hidden",
          }}
        >
          {subjectId}
        </Typography>
      </ButtonBase>
      <Menu
        id="profile-menu"
        anchorEl={anchorEl}
        open={open}
        onClose={closeMenu}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
        // The top bar sits one step above theme.zIndex.modal (so it stays
        // usable over dialogs), so its menu must go one step further to not
        // slide under the bar.
        sx={{ zIndex: (theme) => theme.zIndex.modal + 2 }}
      >
        <MenuItem
          disabled
          dense
          sx={{ "&.Mui-disabled": { opacity: 1 } }}
        >
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ overflowWrap: "anywhere" }}
          >
            {subjectId}
          </Typography>
        </MenuItem>
        <Divider />
        <MenuItem onClick={handleLogout} disabled={logout.isPending}>
          <ListItemIcon>
            <LogoutIcon fontSize="small" />
          </ListItemIcon>
          Log Out
        </MenuItem>
      </Menu>
    </>
  );
}
