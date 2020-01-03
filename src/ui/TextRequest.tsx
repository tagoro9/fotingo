import React = require('react');

import { Box } from 'ink';
import TextInput from 'ink-text-input';
import { useState } from 'react';

import { RequestProps } from './props';
import { Question } from './Question';

export function TextRequest({ onSubmit, request }: RequestProps): JSX.Element {
  const [value, setValue] = useState<string>('');
  return (
    <Box>
      <Question {...request} />
      <TextInput
        mask={request.requestType === 'password' ? '*' : undefined}
        value={value}
        onChange={setValue}
        onSubmit={(): void => onSubmit(value)}
      />
    </Box>
  );
}
