export interface HttpError {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly body: any;
  message: string;
  readonly status: number;
}

export class HttpErrorImpl extends Error implements HttpError {
  public readonly status: number;
  public readonly body: Record<string, unknown>;

  constructor(message: string, status: number, body: Record<string, unknown>) {
    super(message);
    Object.setPrototypeOf(this, new.target.prototype);
    this.status = status;
    this.body = body;
  }
}
