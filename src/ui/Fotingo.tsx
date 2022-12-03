import React = require('react');

import { Footer } from './Footer';
import { InputRequest } from './InputRequest';
import { Messages } from './Messages';
import { FotingoProps } from './props';
import { useCmd } from './useCmd';

export function Fotingo({
  cmd,
  isDebugging,
  messenger,
  programStartTime,
  showFooter = true,
}: FotingoProps): JSX.Element {
  const { executionTime, isDone, isInThread, messages, request, sendRequestData } = useCmd(
    messenger,
    programStartTime,
    cmd,
  );

  return (
    <>
      <Messages
        isDebugging={isDebugging}
        isDone={isDone}
        isRequesting={request !== undefined}
        isInThread={isInThread}
        messages={messages}
      />
      {request && <InputRequest request={request} onSubmit={sendRequestData} />}
      {!request && executionTime && showFooter && <Footer executionTime={executionTime} />}
    </>
  );
}
