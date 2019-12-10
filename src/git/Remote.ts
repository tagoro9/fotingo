import { Issue, Release, ReleaseNotes } from 'src/types';
import { BranchInfo } from './Git';

export interface Remote {
  createPullRequest: (data: PullRequestData) => Promise<PullRequest>;
  getLabels: () => Promise<Label[]>;
  createRelease: (release: Release, notes: ReleaseNotes) => Promise<JointRelease>;
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
  name?: string;
  login: string;
}

export interface RemoteRelease {
  id: number;
  url: string;
}

export interface GitRemote {
  owner: string;
  name: string;
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
  number: number;
  url: string;
  issues: Issue[];
}
