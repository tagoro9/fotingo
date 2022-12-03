import React = require('react');

import { Box, Text } from 'ink';
import Spinner from 'ink-spinner';
import { get as getEmoji } from 'node-emoji';

import { MessageProps } from './props';

const MESSAGE_TYPE_TO_EMOJI: Record<MessageProps['message']['type'], string | undefined> = {
  error: '💥',
  info: '📝',
  request: '❔',
  status: undefined,
};

export function Message({ isDone = false, isLast = false, message }: MessageProps): JSX.Element {
  return (
    <Box>
      <Box marginRight={2}>
        {!isDone && message.showSpinner && isLast ? (
          <Text color="cyan">
            <Spinner type="dots" />
          </Text>
        ) : (
          <Text>
            {(message.emoji && getEmoji(message.emoji)) || MESSAGE_TYPE_TO_EMOJI[message.type]}
          </Text>
        )}
      </Box>
      <Box>
        <Text color={message.type === 'error' ? 'red' : undefined}>{message.message}</Text>
        {message.detail && (
          <Box marginLeft={1}>
            <Text color={message.type === 'request' ? 'gray' : undefined}>{message.detail}</Text>
          </Box>
        )}
      </Box>
    </Box>
  );
}
