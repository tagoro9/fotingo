import React = require('react');

import { pointer } from 'figures';
import { Box, Text } from 'ink';

interface ControlledSelectInputProperties {
  items: Array<{
    key?: React.Key;
    label: string;
    value: React.Key;
  }>;
  selectedIndex?: number;
}

interface IndicatorProperties {
  isSelected: boolean;
}

interface ItemProperties {
  isSelected: boolean;
  label: string;
}

function Indicator({ isSelected }: IndicatorProperties): JSX.Element {
  return (
    <Box marginRight={1}>{isSelected ? <Text color="blue">{pointer}</Text> : <Text> </Text>}</Box>
  );
}

function Item({ isSelected, label }: ItemProperties): JSX.Element {
  return <Text color={isSelected ? 'blue' : undefined}>{label}</Text>;
}

export function ControlledSelectInput({
  items,
  selectedIndex,
}: ControlledSelectInputProperties): JSX.Element {
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
