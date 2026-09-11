import Box from "@mui/material/Box";
import CssBaseline from "@mui/material/CssBaseline";
import { ThemeProvider } from "@mui/material/styles";
import Toolbar from "@mui/material/Toolbar";
import { useState, type JSX } from "react";

import { ChatPage } from "@/pages/chat";

import { theme } from "./theme";
import { NavigationDrawer } from "./ui/NavigationDrawer";
import { TopBar } from "./ui/TopBar";

/**
 * The application shell: an MUI AppBar + Drawer with one navigation entry.
 * See docs/specs/orchestration.md section 7.
 *
 * The previous DashboardLayout-based shell was replaced with a hand-built
 * MUI shell: that dependency's peer range topped out at @mui/material ^7,
 * this repository is on ^9, and no MUI 8/9 compatible release of it exists.
 */
export function App(): JSX.Element {
  const [open, setOpen] = useState(true);

  const toggleDrawer = (): void => {
    setOpen((current) => !current);
  };

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box sx={{ display: "flex" }}>
        <TopBar onToggleDrawer={toggleDrawer} />
        <NavigationDrawer open={open} />
        <Box component="main" sx={{ flexGrow: 1 }}>
          <Toolbar />
          <ChatPage />
        </Box>
      </Box>
    </ThemeProvider>
  );
}
