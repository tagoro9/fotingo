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

export function Message({
  isDone = false,
  isLast = false,
  message,
  useRawOutput = false,
}: MessageProps): JSX.Element {
  const spinnerOrEmoji =
    useRawOutput && message.type !== 'error' ? undefined : (
      <Box>
        {!isDone && message.showSpinner && isLast ? (
          <Box marginRight={1}>
            <Text color="cyan">
              <Spinner type="dots" />
            </Text>
          </Box>
        ) : (
          <Text>
            {(message.emoji && getEmoji(message.emoji)) || MESSAGE_TYPE_TO_EMOJI[message.type]}
          </Text>
        )}
      </Box>
    );
  const messageColor = useRawOutput || message.type !== 'error' ? undefined : 'red';
  const messageDetailColor = useRawOutput || message.type !== 'request' ? undefined : 'gray';
  const wrap = useRawOutput ? 'end' : undefined;
  return (
    <Box>
      {spinnerOrEmoji}
      <Box>
        <Text color={messageColor} wrap={wrap}>
          {message.message}
        </Text>
        {message.detail && (
          <Box marginLeft={1}>
            <Text color={messageDetailColor} wrap={wrap}>
              {message.detail}
            </Text>
          </Box>
        )}
      </Box>
    </Box>
  );
}
