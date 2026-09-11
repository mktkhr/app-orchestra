import Box from "@mui/material/Box";
import { useTheme } from "@mui/material/styles";
import Toolbar from "@mui/material/Toolbar";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useState, type JSX } from "react";

import { ErrorBoundary } from "./ErrorBoundary";
import { MainContent } from "./MainContent";
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

  // Open by default only where the drawer sits beside the content. On a
  // phone it covers what the person came to read, so it starts closed and
  // waits to be asked for.
  const [open, setOpen] = useState(beside);

  // Crossing the breakpoint changes what the drawer is: a column of the
  // layout above it, a sheet over the content below it. Follow the width,
  // rather than leave a sheet open over a page nobody asked to cover.
  // Adjusted during render, which is how React says to reset state when the
  // thing it depends on changes - an effect would render the wrong drawer
  // once before correcting it.
  const [wasBeside, setWasBeside] = useState(beside);

  if (wasBeside !== beside) {
    setWasBeside(beside);
    setOpen(beside);
  }

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
          <MainContent />
        </ErrorBoundary>
      </Box>
    </Box>
  );
}
