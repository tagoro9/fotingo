// eslint-disable-next-line unicorn/prevent-abbreviations
import { Observable } from 'rxjs';
import { Message, Messenger, Request, SelectRequest } from 'src/io/messenger';

export interface RequestProps {
  onSubmit: (value: string) => void;
  request: Request | SelectRequest;
}

export interface SelectRequestProps extends Omit<RequestProps, 'request'> {
  request: SelectRequest;
}

export interface MessageProps {
  isDone?: boolean;
  isLast?: boolean;
  message: Omit<Message, 'message'> & { message: string | Element };
}

export interface MessagesProps {
  isDebugging: boolean;
  isDone: boolean;
  isInThread: boolean;
  isRequesting: boolean;
  messages: Array<Omit<Message, 'message'> & { message: string | Element }>;
}

export interface FotingoProps {
  cmd: () => Observable<unknown>;
  isDebugging: boolean;
  messenger: Messenger;
}
