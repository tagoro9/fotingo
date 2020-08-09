import React = require('react');

import { Box, useInput } from 'ink';
import InkTextInput from 'ink-text-input';
import { useState } from 'react';

import { ControlledSelectInput } from './ControlledSelectInput';
import { SelectRequestProps } from './props';
import { Question } from './Question';

/**
 * Component that given a select request, will display the options allowing the user to select
 * the one they want. It allows users to search among the options if the allowTextSearch option is set
 */
export function SelectRequest({ onSubmit, request }: SelectRequestProps): JSX.Element {
  const useTextSearch = request.allowTextSearch === true;
  const [searchText, setSearchText] = useState('');
  const [items, setItems] = useState(request.options);
  const [selectedIndex, setSelectedIndex] = useState(0);
  useInput((_, key) => {
    if (key.downArrow) {
      setSelectedIndex((selectedIndex + 1) % items.length);
    }
    if (key.return) {
      onSubmit(items[selectedIndex]?.value);
    }
    if (key.upArrow) {
      setSelectedIndex((((selectedIndex - 1) % items.length) + items.length) % items.length);
    }
  });

  const onInputChange = (text: string): void => {
    const escapedText = text.replace(/\\u001b/g, '');
    setSearchText(escapedText);
    if (escapedText && escapedText.trim() !== '') {
      setItems(
        request.options.filter((option) =>
          option.label.toLowerCase().includes(escapedText.toLowerCase()),
        ),
      );
    } else {
      setItems(request.options);
    }
    setSelectedIndex(0);
  };

  return (
    <Box flexDirection="column">
      <Box>
        <Question {...request} />
        {useTextSearch ? (
          <InkTextInput showCursor={true} value={searchText} onChange={onInputChange} />
        ) : undefined}
      </Box>
      <Box marginBottom={1} marginLeft={1} marginTop={1}>
        <ControlledSelectInput items={items} selectedIndex={selectedIndex} />
      </Box>
    </Box>
  );
}
