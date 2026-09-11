import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { Conversation } from "@/features/conversation";

/** The chat screen: title plus the conversation (turn list, question input, welcome state). */
export function ChatPage(): JSX.Element {
  return (
    <Stack spacing={3} sx={{ p: 3 }}>
      <Typography variant="h5" component="h1">
        チャット
      </Typography>
      <Conversation />
    </Stack>
  );
}
