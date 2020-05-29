import { useEffect, useRef, useState } from 'react';
import { Observable } from 'rxjs';
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
  const messengerRef = useRef(messenger);
  const addMessageRef = useRef(addMessage);
  useEffect(() => {
    messengerRef.current.onMessage((message) => {
      if (isRequest(message)) {
        setRequest(message);
      } else if (isStatus(message)) {
        setInThread(message.inThread);
      } else {
        addMessageRef.current(message);
      }
    });
  }, [addMessageRef, messenger, setRequest, setInThread]);
}

function useCmdRunner(
  cmd: () => Observable<unknown>,
  addMessage: Setter<Message>,
  setInThread: Setter<boolean>,
): number | undefined {
  const [done, setDone] = useState<number>();
  const addMessageRef = useRef(addMessage);
  const setInThreadRef = useRef(setInThread);
  useEffect(() => {
    const time = Date.now();
    cmd()
      .toPromise()
      .catch((e) => {
        // Exit thread mode if there was an error so it shows up
        setInThreadRef.current(false);
        addMessageRef.current({
          message: (e.code && ERROR_CODE_TO_MESSAGE[e.code]) || e.message,
          showSpinner: false,
          type: MessageType.ERROR,
        });
      })
      .finally(() => setDone(Date.now() - time));
  }, [addMessageRef, cmd, setInThreadRef]);

  return done;
}

export function useCmd(messenger: Messenger, cmd: () => Observable<unknown>): CmdStatus {
  const [messages, addMessage] = useMessages();
  const [request, setRequest] = useState<Request>();
  const [isInThread, setInThread] = useState<boolean>(false);

  useMessenger(messenger, addMessage, setRequest, setInThread);
  const executionTime = useCmdRunner(cmd, addMessage, setInThread);

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
