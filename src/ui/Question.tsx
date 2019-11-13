import React = require('react');

import { Box, Color } from 'ink';

interface QuestionProps {
  hint?: string;
  message: string;
}

export function Question({ hint, message }: QuestionProps) {
  return (
    <Box marginRight={1}>
      <Box marginRight={2}>❔</Box>
      <Box>{message}</Box>
      {hint && (
        <Box marginLeft={1}>
          <Color grey>[{hint}]</Color>
        </Box>
      )}
    </Box>
  );
}
