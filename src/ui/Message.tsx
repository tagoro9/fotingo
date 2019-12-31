import React = require('react');

import { Box, Color } from 'ink';
import Spinner from 'ink-spinner';
import { get as getEmoji } from 'node-emoji';

import { MessageProps } from './props';

const MESSAGE_TYPE_TO_EMOJI: Record<MessageProps['message']['type'], string | undefined> = {
  error: '💥',
  info: '📝',
  request: '❔',
  status: undefined,
};

export function Message({ isDone = false, isLast = false, message }: MessageProps) {
  return (
    <Box>
      <Box marginRight={2}>
        {!isDone && message.showSpinner && isLast ? (
          <Color cyan>
            <Spinner type="dots" />
          </Color>
        ) : (
          (message.emoji && getEmoji(message.emoji)) || MESSAGE_TYPE_TO_EMOJI[message.type]
        )}
      </Box>
      <Box>
        <Color red={message.type === 'error'}>{message.message}</Color>
        {message.detail && (
          <Box marginLeft={1}>
            <Color grey={message.type === 'request'}>{message.detail}</Color>
          </Box>
        )}
      </Box>
    </Box>
  );
}
