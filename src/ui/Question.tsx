import React = require('react');

import { Box, Text } from 'ink';

interface QuestionProperties {
  hint?: string;
  message: string;
}

export function Question({ hint, message }: QuestionProperties): JSX.Element {
  return (
    <Box marginRight={1}>
      <Box marginRight={2}>
        <Text>❔</Text>
      </Box>
      <Box>
        <Text>{message}</Text>
      </Box>
      {hint && (
        <Box marginLeft={1}>
          <Text color="grey>">[{hint}]</Text>
        </Box>
      )}
    </Box>
  );
}
