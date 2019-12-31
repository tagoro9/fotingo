import { Issue, Release, ReleaseNotes } from 'src/types';

import { BranchInfo } from './Git';

export interface Remote {
  createPullRequest: (data: PullRequestData) => Promise<PullRequest>;
  createRelease: (release: Release, notes: ReleaseNotes) => Promise<JointRelease>;
  getLabels: () => Promise<Label[]>;
  getPossibleReviewers: () => Promise<Reviewer[]>;
}

export interface JointRelease {
  release: Release;
  remoteRelease: RemoteRelease;
}

export interface Label {
  id: number;
  name: string;
}

export interface Reviewer {
  login: string;
  name?: string;
}

export interface RemoteRelease {
  id: number;
  url: string;
}

export interface GitRemote {
  name: string;
  owner: string;
  repo: string;
}

// TODO Rename to CreatePullRequest
export interface PullRequestData {
  branchInfo: BranchInfo;
  issues: Issue[];
  labels?: string[];
  reviewers?: string[];
  useDefaults: boolean;
}

export interface PullRequest {
  issues: Issue[];
  number: number;
  url: string;
}
