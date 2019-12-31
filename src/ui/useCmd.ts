import { useEffect, useState } from 'react';
import { Observable } from 'rxjs';
import { Message, MessageType, Messenger, Request, Status } from 'src/io/messenger';

import { ERROR_CODE_TO_MESSAGE } from './errorCodeToMessage';

type Setter<T> = (data: T) => void;

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
    (message: Message) => setMessages(currentMessages => [...currentMessages, message]),
  ];
}

function useMessenger(
  messenger: Messenger,
  addMessage: Setter<Message>,
  setRequest: Setter<Request>,
  setInThread: Setter<boolean>,
) {
  useEffect(() => {
    messenger.onMessage(message => {
      if (isRequest(message)) {
        setRequest(message);
      } else if (isStatus(message)) {
        setInThread(message.inThread);
      } else {
        addMessage(message);
      }
    });
  }, []);
}

function useCmdRunner(cmd: () => Observable<any>, addMessage: Setter<Message>) {
  const [done, setDone] = useState<number>();
  useEffect(() => {
    const time = Date.now();
    cmd()
      .toPromise()
      .catch(e =>
        addMessage({
          message: (e.code && ERROR_CODE_TO_MESSAGE[e.code]) || e.message,
          showSpinner: false,
          type: MessageType.ERROR,
        }),
      )
      .finally(() => setDone(Date.now() - time));
  }, []);

  return done;
}

export function useCmd(messenger: Messenger, cmd: () => Observable<any>) {
  const [messages, addMessage] = useMessages();
  const [request, setRequest] = useState<Request>();
  const [isInThread, setInThread] = useState<boolean>(false);

  useMessenger(messenger, addMessage, setRequest, setInThread);
  const executionTime = useCmdRunner(cmd, addMessage);

  const sendRequestData = (value: any) => {
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
