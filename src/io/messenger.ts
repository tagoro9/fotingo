import { boundMethod } from 'autobind-decorator';
import { Observable, Subject, Subscription } from 'rxjs';
import { mapTo, take } from 'rxjs/operators';

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
  allowTextSearch?: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  options: Array<{ label: string; value: any }>;
}

/**
 * Class used to allow the commands to send messages / data requests to
 * the UI
 */
export class Messenger {
  private subject: Subject<Message | Request | Status>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
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

  /**
   * Pause execution until the user hits enter
   */
  public pause(): Observable<void> {
    return this.request('Press enter to continue', { options: [] }).pipe(mapTo(undefined));
  }

  public request<T>(
    question: string,
    options: {
      allowTextSearch?: boolean;
      hint?: string;
      options?: Array<{ label: string; value: T }>;
    } = {},
  ): Observable<T> {
    const request$ = this.requestSubject.pipe(take(1));
    this.subject.next({
      detail: options.hint,
      message: question,
      requestType: options.options ? RequestType.SELECT : RequestType.TEXT,
      showSpinner: false,
      type: MessageType.REQUEST,
      ...(options.options
        ? { allowTextSearch: options.allowTextSearch, options: options.options }
        : {}),
    });
    return request$;
  }

  @boundMethod
  public error(error: Error): void {
    this.subject.error(error);
  }

  public onMessage(function_: (message: Message) => void): Subscription {
    return this.subject.subscribe({
      next: function_,
    });
  }

  public stop(): void {
    this.subject.complete();
  }
}
