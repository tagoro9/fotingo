/**
 * Git config
 */
export interface GitConfig {
  /**
   * Name of the remote to connect to
   */
  remote: string;
  /**
   * Name of the base branch to use when creating a new branch
   */
  baseBranch: string;

  /**
   * Template to use for creating branch names
   */
  branchTemplate: string;
}

export interface GithubConfig {
  authToken: string;
  baseBranch: string;
  owner: string;
  repo: string;
  pullRequestTemplate: string;
}
