import React = require('react');

import { Box } from 'ink';
import SelectInput, { Item } from 'ink-select-input';

import { SelectRequestProps } from './props';
import { Question } from './Question';

export function SelectRequest({ onSubmit, request }: SelectRequestProps): JSX.Element {
  return (
    <Box flexDirection="column">
      <Question {...request} />
      <SelectInput
        items={request.options}
        onSelect={(item: Item): void => void onSubmit(String(item.value))}
      />
    </Box>
  );
}
