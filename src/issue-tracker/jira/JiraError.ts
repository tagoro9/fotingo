export interface JiraError {
  readonly code: string | number;
  readonly message: string;
}

export class JiraErrorImpl extends Error implements JiraError {
  public readonly code: string | number;

  constructor(message: string, code: string | number) {
    super(message);
    Object.setPrototypeOf(this, new.target.prototype);
    this.code = code;
  }
}
