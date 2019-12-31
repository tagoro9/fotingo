import React = require('react');

import { Box } from 'ink';
import SelectInput from 'ink-select-input';

import { SelectRequestProps } from './props';
import { Question } from './Question';

export function SelectRequest({ onSubmit, request }: SelectRequestProps) {
  return (
    <Box flexDirection="column">
      <Question {...request} />
      <SelectInput items={request.options} onSelect={item => onSubmit(String(item.value))} />
    </Box>
  );
}
