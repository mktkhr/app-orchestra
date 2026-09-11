import MenuIcon from "@mui/icons-material/Menu";
import AppBar from "@mui/material/AppBar";
import IconButton from "@mui/material/IconButton";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

interface TopBarProps {
  readonly onToggleDrawer: () => void;
}

/** The application's top bar: the drawer toggle and the app name. */
export function TopBar({ onToggleDrawer }: TopBarProps): JSX.Element {
  return (
    <AppBar position="fixed" sx={{ zIndex: (appTheme) => appTheme.zIndex.drawer + 1 }}>
      <Toolbar sx={{ display: "flex", alignItems: "center" }}>
        <IconButton
          color="inherit"
          aria-label="ナビゲーションの開閉"
          onClick={onToggleDrawer}
          edge="start"
          sx={{ mr: 2 }}
        >
          <MenuIcon />
        </IconButton>
        <Typography variant="h6" component="div" noWrap sx={{ lineHeight: 1 }}>
          app-orchestra
        </Typography>
      </Toolbar>
    </AppBar>
  );
}
