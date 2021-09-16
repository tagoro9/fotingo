import React = require('react');

import { Box, Color } from 'ink';

interface QuestionProperties {
  hint?: string;
  message: string;
}

export function Question({ hint, message }: QuestionProperties): JSX.Element {
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
