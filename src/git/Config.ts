/**
 * Git config
 */
export interface GitConfig {
  /**
   * Name of the base branch to use when creating a new branch
   */
  baseBranch: string;
  /**
   * Template to use for creating branch names
   */
  branchTemplate: string;

  /**
   * Name of the remote to connect to
   */
  remote: string;
}

export interface GithubConfig {
  authToken: string;
  owner: string;
  pullRequestTemplate: string;
  repo: string;
}
