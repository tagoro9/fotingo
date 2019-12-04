export enum GitErrorType {
  BRANCH_ALREADY_EXISTS,
  NOT_A_GIT_REPO,
}

export interface GitError {
  readonly code: GitErrorType;
  readonly message: string;
}

export class GitErrorImpl extends Error implements GitError {
  public readonly code: GitErrorType;

  constructor(message: string, code: GitErrorType) {
    super(message);
    Object.setPrototypeOf(this, new.target.prototype);
    this.code = code;
  }
}
