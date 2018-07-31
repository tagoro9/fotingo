export interface HttpError {
  message: string;
  readonly status: number;
  readonly body: any;
}

export class HttpErrorImpl extends Error implements HttpError {
  public readonly status: number;
  public readonly body: object;

  constructor(message: string, status: number, body: object) {
    super(message);
    Object.setPrototypeOf(this, new.target.prototype);
    this.status = status;
    this.body = body;
  }
}
