import { Observable } from 'rxjs';
import { FotingoArguments } from 'src/commands/FotingoArguments';
import { Message, Messenger, Request, SelectRequest } from 'src/io/messenger';

export interface RequestProps {
  request: Request | SelectRequest;
  onSubmit: (value: string) => void;
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
  args: FotingoArguments;
  isDebugging: boolean;
  messenger: Messenger;
  cmd: () => Observable<any>;
}
