import Box from "@mui/material/Box";
import { useTheme } from "@mui/material/styles";
import Toolbar from "@mui/material/Toolbar";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useState, type JSX } from "react";

import { ChatPage } from "@/pages/chat";

import { ErrorBoundary } from "./ErrorBoundary";
import { NavigationDrawer } from "./NavigationDrawer";
import { TopBar } from "./TopBar";

/**
 * The AppBar + Drawer shell around the one page.
 *
 * Separate from App so it sits below ThemeProvider and can read the theme's
 * own breakpoints, rather than naming a width of its own.
 */
export function Shell(): JSX.Element {
  const beside = useMediaQuery(useTheme().breakpoints.up("sm"));
  const [open, setOpen] = useState(true);

  const toggleDrawer = (): void => {
    setOpen((current) => !current);
  };

  const closeDrawer = (): void => {
    setOpen(false);
  };

  return (
    <Box sx={{ display: "flex" }}>
      <TopBar onToggleDrawer={toggleDrawer} />
      <NavigationDrawer open={open} beside={beside} onClose={closeDrawer} />
      <Box component="main" sx={{ flexGrow: 1, minWidth: 0 }}>
        <Toolbar />
        <ErrorBoundary>
          <ChatPage />
        </ErrorBoundary>
      </Box>
    </Box>
  );
}
