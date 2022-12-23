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
  useRawOutput = false,
}: MessagesProps): JSX.Element {
  const actualMessages = messages.filter(
    (message) =>
      message.showInRawMode || ['request', 'error'].includes(message.type) || !useRawOutput,
  );
  const pastMessages = init(actualMessages);
  const lastMessage = last(actualMessages);
  const staticMessages =
    isDebugging && lastMessage !== undefined ? [...pastMessages, lastMessage] : pastMessages;
  return (
    <Box flexDirection="column">
      <Static items={staticMessages}>
        {(message, id) => <Message key={id} message={message} useRawOutput={useRawOutput} />}
      </Static>
      {lastMessage && !isDebugging && !isRequesting && !isInThread && (
        <Message isDone={isDone} isLast={true} message={lastMessage} useRawOutput={useRawOutput} />
      )}
    </Box>
  );
}
