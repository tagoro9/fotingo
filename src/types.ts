import { ParsedCommit } from 'conventional-commits-parser';

/**
 * Config
 */

interface GitConfig {
  baseBranch: string;
  branchTemplate: string;
  remote: string;
}

interface RemoteConfig {
  authToken: string;
  owner: string;
  pullRequestTemplate: string;
  repo: string;
}

export interface RemoteUser {
  login: string;
}

export interface TrackerConfig {
  root: string;
  status: Record<IssueStatus, RegExp>;
  user: {
    login: string;
    token: string;
  };
}

export interface ReleaseConfig {
  template: string;
}

export interface Config {
  git: GitConfig;
  github: RemoteConfig;
  jira: TrackerConfig;
  release: ReleaseConfig;
}

export interface DefaultConfig {
  git: {
    baseBranch: string;
    branchTemplate: string;
    remote: string;
  };
  github: {
    pullRequestTemplate: string;
  };
  jira: {
    releaseTemplate: string;
  };
}

/**
 * Tracker
 */

export enum IssueType {
  BUG = 'Bug',
  FEATURE = 'Feature',
  SPIKE = 'Spike',
  STORY = 'Story',
  SUB_TASK = 'Sub-task',
  TASK = 'Task',
  TECH_DEBT = 'Tech-debt',
}

export enum IssueStatus {
  BACKLOG = 'BACKLOG',
  DONE = 'DONE',
  IN_PROGRESS = 'IN_PROGRESS',
  IN_REVIEW = 'IN_REVIEW',
  SELECTED_FOR_DEVELOPMENT = 'SELECTED_FOR_DEVELOPMENT',
}

// TODO This should be Jira only
export interface User {
  accountId: string;
  displayName: string;
  groups: {
    items: {
      name: string;
    };
  };
}

export interface Issue {
  description: string;
  id: number;
  key: string;
  project: {
    id: string;
    key: string;
  };
  sanitizedSummary: string;
  summary: string;
  type: IssueType;
  url: string;
}

interface IssueCommentAuthor {
  active: boolean;
  displayName: string;
  name: string;
}

export interface IssueComment {
  author: IssueCommentAuthor;
  body: string;
  created: string;
  id: string;
  updateAuthor: IssueCommentAuthor;
  updated: string;
}

interface CreateIssueCommon {
  description?: string;
  labels?: string[];
  title: string;
  type: IssueType;
}

export interface CreateStandaloneIssue extends CreateIssueCommon {
  project: string;
}

export interface CreateSubTask extends CreateIssueCommon {
  parent: string;
}

export type CreateIssue = CreateStandaloneIssue | CreateSubTask;

export interface CreateRelease {
  issues: Issue[];
  name: string;
  submitRelease: boolean;
  useDefaults: boolean;
}

export interface GetIssue {
  id: string;
}

export interface ReleaseNotes {
  body: string;
  title: string;
}

export interface Release {
  id: string;
  issues: Issue[];
  name: string;
  // notes: ReleaseNotes;
  url?: string;
}

/**
 * Git
 */

interface GitRemote {
  name: string;
  refs: {
    fetch: string;
    push: string;
  };
}

interface GitLogLine {
  author_email: string;
  author_name: string;
  date: string;
  hash: string;
  message: string;
}

export interface GitLog {
  all: GitLogLine[];
  latest: GitLogLine;
  total: number;
}

interface CommitIssue {
  issue: string;
  raw: 'string';
}

export interface BranchInfo {
  commits: ParsedCommit[];
  issues: CommitIssue[];
  name: string;
}

export interface Vcs {
  createBranchAndStashChanges(branchName: string): Promise<void>;
  doesBranchExist(branchName: string): Promise<boolean>;
  doesCurrentBranchExistInRemote(): Promise<boolean>;
  getBranchInfo(): Promise<BranchInfo>;
  getBranchNameForIssue(issue: Issue): string;
  getRemote(name: string): Promise<GitRemote>;
  getRootDir(): Promise<string>;
  push(): Promise<void>;
}

/**
 * Remote
 */

export interface CreatePullRequest {
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

export interface JointRelease {
  release: Release;
  remoteRelease?: RemoteRelease;
}

export interface Remote {
  createPullRequest: (data: CreatePullRequest) => Promise<PullRequest>;
  createRelease: (release: Release) => Promise<JointRelease>;
  getLabels: () => Promise<Label[]>;
  getPossibleReviewers: () => Promise<Reviewer[]>;
}

/**
 * CLI types
 */

export interface ReviewData {
  branch?: string;
  isDraft: boolean;
  labels?: string[];
  reviewers?: string[];
  tracker: {
    enabled: boolean;
  };
  useDefaults: boolean;
}

export interface Review {
  // TODO Rename comments. Its weird
  comments: Issue[];
  pullRequest: PullRequest;
}

export interface StartData {
  git: {
    createBranch: boolean;
  };
  issue: CreateStandaloneIssue | CreateSubTask | GetIssue | undefined;
}

export interface ReleaseData {
  issues: string[];
  name: string;
  tracker: {
    enabled: boolean;
  };
  useDefaults: boolean;
  vcs: {
    enabled: boolean;
  };
}

export interface LocalChanges {
  branchInfo: BranchInfo;
  issues: Issue[];
}

export interface OpenData {
  source: 'pr' | 'jira' | 'repo';
}
