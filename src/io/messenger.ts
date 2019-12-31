import { boundMethod } from 'autobind-decorator';
import { Observable, Subject } from 'rxjs';
import { take } from 'rxjs/operators';

export enum MessageType {
  ERROR = 'error',
  INFO = 'info',
  REQUEST = 'request',
  STATUS = 'status',
}

export enum RequestType {
  PASSWORD = 'password',
  SELECT = 'select',
  TEXT = 'text',
}

export enum Emoji {
  ARROW_UP = 'arrow_up',
  BOOKMARK = 'books',
  BOOKS = 'books',
  BOOM = 'boom',
  BUG = 'bug',
  LINK = 'link',
  MAG_RIGHT = 'mag_right',
  OK = 'ok_hand',
  PENCIL = 'pencil2',
  QUESTION = 'grey_question',
  SHIP = 'ship',
  TADA = 'tada',
}

export interface Message {
  detail?: string;
  emoji?: Emoji;
  message: string;
  showSpinner: boolean;
  type: MessageType;
}

export interface Request extends Message {
  requestType: RequestType;
  type: MessageType.REQUEST;
}

export interface Status extends Message {
  inThread: boolean;
}

export interface SelectRequest extends Request {
  options: Array<{ label: string; value: any }>;
}

/**
 * Class used to allow the commands to send messages / data requests to
 * the UI
 */
export class Messenger {
  private subject: Subject<Message | Request | Status>;
  private requestSubject: Subject<any>;

  constructor() {
    this.subject = new Subject();
    this.requestSubject = new Subject();
  }

  @boundMethod
  public emit(message: string, emoji?: Emoji): void {
    this.subject.next({ emoji, message, type: MessageType.INFO, showSpinner: true });
  }

  @boundMethod
  public inThread(value: boolean): void {
    this.subject.next({
      inThread: value,
      message: 'It does not matter',
      showSpinner: false,
      type: MessageType.STATUS,
    });
  }

  public send(data: string): void {
    this.requestSubject.next(data);
  }

  public request<T>(
    question: string,
    options: { hint?: string; options?: Array<{ label: string; value: T }> } = {},
  ): Observable<T> {
    const request$ = this.requestSubject.pipe(take(1));
    this.subject.next({
      detail: options.hint,
      message: question,
      requestType: options.options ? RequestType.SELECT : RequestType.TEXT,
      showSpinner: false,
      type: MessageType.REQUEST,
      ...(options.options ? { options: options.options } : {}),
    });
    return request$;
  }

  @boundMethod
  public error(error: Error): void {
    this.subject.error(error);
  }

  public onMessage(fn: (msg: Message) => void): void {
    this.subject.subscribe({
      next: fn,
    });
  }

  public stop(): void {
    this.subject.complete();
  }
}
