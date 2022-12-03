export interface GithubError {
  readonly code: string | number;
  readonly message: string;
}

export interface GithubRequestError extends Error {
  code: number;
  name: 'HttpError';
  response?: object;
  status: number;
}

export class GithubErrorImpl extends Error implements GithubError {
  public readonly code: string | number;

  constructor(message: string, code: string | number) {
    super(message);
    Object.setPrototypeOf(this, new.target.prototype);
    this.code = code;
  }
}
