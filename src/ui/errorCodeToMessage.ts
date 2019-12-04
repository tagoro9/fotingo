import { GitErrorType } from 'src/git/GitError';

export const ERROR_CODE_TO_MESSAGE: { [k: number]: string } = {
  [GitErrorType.BRANCH_ALREADY_EXISTS]: 'It looks like there is a branch already for this issue',
  [GitErrorType.NOT_A_GIT_REPO]: 'It looks like you run fotingo outside of a git repository',
};
