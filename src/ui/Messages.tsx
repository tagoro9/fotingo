import React = require('react');

import { Box, Static } from 'ink';
import { init, last } from 'ramda';

import { Message } from './Message';
import { MessagesProps } from './props';

export function Messages({
  isDebugging,
  isDone,
  isInThread,
  isRequesting,
  messages,
}: MessagesProps): JSX.Element {
  const pastMessages = init(messages);
  const lastMessage = last(messages);
  const staticMessages =
    isDebugging && lastMessage !== undefined ? [...pastMessages, lastMessage] : pastMessages;
  return (
    <Box flexDirection="column">
      <Static>
        {staticMessages.map((message, id) => (
          <Message key={id} message={message} />
        ))}
      </Static>
      {lastMessage && !isDebugging && !isRequesting && !isInThread && (
        <Message isDone={isDone} isLast={true} message={lastMessage} />
      )}
    </Box>
  );
}
