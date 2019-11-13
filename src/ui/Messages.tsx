import React = require('react');

import { Box, Static } from 'ink';
import { init, last } from 'ramda';
import { Message } from './Message';
import { MessagesProps } from './props';

export function Messages({ isDone, isInThread, isRequesting, messages }: MessagesProps) {
  const pastMessages = init(messages);
  const lastMessage = last(messages);
  return (
    <Box flexDirection="column">
      <Static>
        {pastMessages.map((msg, id) => (
          <Message key={id} message={msg} />
        ))}
      </Static>
      {lastMessage && !isRequesting && !isInThread && (
        <Message isDone={isDone} isLast={true} message={lastMessage} />
      )}
    </Box>
  );
}
