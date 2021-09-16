import humanize from 'humanize-duration';
import { Box } from 'ink';
import React = require('react');

interface FooterProperties {
  executionTime: number;
}

export function Footer({ executionTime }: FooterProperties): JSX.Element {
  return (
    <Box marginTop={1}>
      <Box marginRight={2}>✨</Box>
      <Box>Done in {humanize(executionTime, { round: true })}.</Box>
    </Box>
  );
}
