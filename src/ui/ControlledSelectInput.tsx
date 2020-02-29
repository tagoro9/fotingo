import React = require('react');

import { pointer } from 'figures';
import { Box, Color } from 'ink';

interface ControlledSelectInputProps {
  items: Array<{
    key?: React.Key;
    label: string;
    value: React.Key;
  }>;
  selectedIndex?: number;
}

interface IndicatorProps {
  isSelected: boolean;
}

interface ItemProps {
  isSelected: boolean;
  label: string;
}

function Indicator({ isSelected }: IndicatorProps): JSX.Element {
  return <Box marginRight={1}>{isSelected ? <Color blue>{pointer}</Color> : ' '}</Box>;
}

function Item({ isSelected, label }: ItemProps): JSX.Element {
  return <Color blue={isSelected}>{label}</Color>;
}

export function ControlledSelectInput({
  items,
  selectedIndex,
}: ControlledSelectInputProps): JSX.Element {
  return (
    <Box flexDirection="column">
      {items.map((item, index) => {
        const isSelected = index === selectedIndex;

        return (
          <Box key={item.key || item.value}>
            <Indicator isSelected={isSelected} />
            <Item {...item} isSelected={isSelected} />
          </Box>
        );
      })}
    </Box>
  );
}
