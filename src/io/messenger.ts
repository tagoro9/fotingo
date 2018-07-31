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
  TEXT = 'text',
  PASSWORD = 'password',
  SELECT = 'select',
}

export enum Emoji {
  PENCIL = 'pencil2',
  ARROW_UP = 'arrow_up',
  BUG = 'bug',
  MAG_RIGHT = 'mag_right',
  BOOKS = 'books',
  BOOKMARK = 'books',
  BOOM = 'boom',
  OK = 'ok_hand',
  QUESTION = 'grey_question',
  LINK = 'link',
  SHIP = 'ship',
  TADA = 'tada',
}

export interface Message {
  detail?: string;
  emoji?: Emoji;
  message: string;
  type: MessageType;
  showSpinner: boolean;
}

export interface Request extends Message {
  type: MessageType.REQUEST;
  requestType: RequestType;
}

export interface Status extends Message {
  inThread: boolean;
}

export interface SelectRequest extends Request {
  options: Array<{ label: string; value: any }>;
}

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
    options: { options?: Array<{ label: string; value: T }>; hint?: string } = {},
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
