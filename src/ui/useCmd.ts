import { useApp } from 'ink';
import { useEffect, useRef, useState } from 'react';
import { lastValueFrom, Observable } from 'rxjs';
import { Message, MessageType, Messenger, Request, Status } from 'src/io/messenger';

import { ERROR_CODE_TO_MESSAGE } from './errorCodeToMessage';

type Setter<T> = (data: T) => void;

interface CmdStatus {
  executionTime: number | undefined;
  isDone: boolean;
  isInThread: boolean;
  messages: Message[];
  request: Request | undefined;
  sendRequestData: (value: string) => void;
}

function isRequest(message: Message): message is Request {
  return message.type === MessageType.REQUEST;
}

function isStatus(message: Message): message is Status {
  return message.type === MessageType.STATUS;
}

function useMessages(): [Message[], (m: Message) => void] {
  const [messages, setMessages] = useState<Message[]>([]);
  return [
    messages,
    (message: Message): void => setMessages((currentMessages) => [...currentMessages, message]),
  ];
}

function useMessenger(
  messenger: Messenger,
  addMessage: Setter<Message>,
  setRequest: Setter<Request>,
  setInThread: Setter<boolean>,
): void {
  const messengerReference = useRef(messenger);
  const addMessageReference = useRef(addMessage);
  useEffect(() => {
    const subscription = messengerReference.current.onMessage((message) => {
      if (isRequest(message)) {
        setRequest(message);
      } else if (isStatus(message)) {
        setInThread(message.inThread);
      } else {
        addMessageReference.current(message);
      }
    });
    return () => {
      subscription.unsubscribe();
    };
  }, [addMessageReference, messenger, setRequest, setInThread]);
}

function useCmdRunner(
  startTime: number,
  cmd: () => Observable<unknown>,
  addMessage: Setter<Message>,
  setInThread: Setter<boolean>,
): number | undefined {
  const app = useApp();
  const errorReference = useRef<Error>();
  const [done, setDone] = useState<number>();
  const addMessageReference = useRef(addMessage);
  const setInThreadReference = useRef(setInThread);
  useEffect(() => {
    const handleError = (error: Error & { code?: number }) => {
      errorReference.current = error;
      // Exit thread mode if there was an error so it shows up
      setInThreadReference.current(false);
      addMessageReference.current({
        message: (error.code && ERROR_CODE_TO_MESSAGE[error.code]) || error.message,
        showSpinner: false,
        type: MessageType.ERROR,
      });
    };
    try {
      lastValueFrom(cmd())
        .catch(handleError)
        .finally(() => {
          setDone(Date.now() - startTime);
          app.exit(errorReference.current);
        });
    } catch (error) {
      handleError(error);
    }
  }, [app, addMessageReference, cmd, setInThreadReference, errorReference, startTime]);

  return done;
}

export function useCmd(
  messenger: Messenger,
  programStartTime: number,
  cmd: () => Observable<unknown>,
): CmdStatus {
  const [messages, addMessage] = useMessages();
  const [request, setRequest] = useState<Request>();
  const [isInThread, setInThread] = useState<boolean>(false);

  useMessenger(messenger, addMessage, setRequest, setInThread);
  const executionTime = useCmdRunner(programStartTime, cmd, addMessage, setInThread);

  const sendRequestData = (value: string): void => {
    if (request) {
      addMessage({
        detail: value,
        message: request.message,
        showSpinner: false,
        type: MessageType.REQUEST,
      });
      setRequest(undefined);
    }
    messenger.send(value);
  };

  return {
    executionTime,
    isDone: executionTime !== undefined,
    isInThread,
    messages,
    request,
    sendRequestData,
  };
}
