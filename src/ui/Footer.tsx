import humanize from 'humanize-duration';
import { Box, Text } from 'ink';
import React = require('react');

interface FooterProperties {
  executionTime: number;
}

export function Footer({ executionTime }: FooterProperties): JSX.Element {
  return (
    <Box marginTop={1}>
      <Box marginRight={2}>
        <Text>✨</Text>
      </Box>
      <Box>
        <Text>Done in {humanize(executionTime, { round: true })}.</Text>
      </Box>
    </Box>
  );
}
