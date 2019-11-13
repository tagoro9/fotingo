import React = require('react');

import { Box } from 'ink';
import TextInput from 'ink-text-input';
import { useState } from 'react';
import { RequestProps } from './props';
import { Question } from './Question';

export function TextRequest({ request, onSubmit }: RequestProps) {
  const [value, setValue] = useState<string>('');
  return (
    <Box>
      <Question {...request} />
      <TextInput
        mask={request.requestType === 'password' ? '*' : undefined}
        value={value}
        onChange={setValue}
        onSubmit={() => onSubmit(value)}
      />
    </Box>
  );
}
