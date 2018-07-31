import * as humanize from 'humanize-duration';
import { Box } from 'ink';
import React = require('react');

interface FooterProps {
  executionTime: number;
}

export function Footer({ executionTime }: FooterProps) {
  return (
    <Box marginTop={1}>
      <Box marginRight={2}>✨</Box>
      <Box>Done in {humanize(executionTime, { round: true })}.</Box>
    </Box>
  );
}
