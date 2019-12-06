import { ParsedCommit } from 'conventional-commits-parser';
import { Observable } from 'rxjs';
import { Arguments } from 'yargs';

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
  baseBranch: string;
  owner: string;
  pullRequestTemplate: string;
  repo: string;
}

interface TrackerConfig {
  releaseTemplate: string;
  root: string;
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
    baseBranch: string;
    pullRequestTemplate: string;
  };
  jira: {
    releaseTemplate: string;
  };
}

/**
 * Tracker
 */

interface IssueTypeData {
  id: number;
  name: string;
  shortName: string;
}

interface IssueTransition {
  id: number;
  name: string;
}

export enum IssueType {
  BUG = 'Bug',
  FEATURE = 'Feature',
  STORY = 'Story',
  SUB_TASK = 'Sub-task',
  TASK = 'Task',
}

export enum IssueStatus {
  BACKLOG = 'BACKLOG',
  DONE = 'DONE',
  IN_PROGRESS = 'IN_PROGRESS',
  IN_REVIEW = 'IN_REVIEW',
  SELECTED_FOR_DEVELOPMENT = 'SELECTED_FOR_DEVELOPMENT',
}

export interface User {
  groups: {
    items: {
      name: string;
    };
  };
  key: string;
}

export interface Issue {
  fields: {
    description?: string;
    issuetype: {
      name: string;
    };
    project: {
      id: string;
    };
    summary: string;
  };
  id: number;
  key: string;
  sanitizedSummary: string;
  transitions: IssueTransition[];
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

export interface IssueEditMeta {
  fields: object;
}

export interface Project {
  id: number;
  issueTypes: { [type in IssueType]: IssueTypeData };
  key: string;
  name: string;
}

export interface CreateIssue {
  description: string;
  project: string;
  type: IssueType;
}

export interface CreateRelease {
  issues: Issue[];
  name: string;
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
  notes: ReleaseNotes;
  url: string;
}

export interface Tracker {
  addCommentToIssue: (issueId: string, comment: string) => Observable<IssueComment>;
  addLabelToIssue: (issueId: string, label: string) => Observable<Issue>;
  createIssue: (data: CreateIssue, user: User) => Observable<Issue>;
  createIssueForCurrentUser: (data: CreateIssue) => Observable<Issue>;
  createRelease: (data: CreateRelease) => Observable<Release>;
  getCurrentUser: () => Observable<User>;
  getIssue: (issueId: string) => Observable<Issue>;
  getIssueEditMeta: (issueId: string) => Observable<IssueEditMeta>;
  isValidIssueName: (name: string) => boolean;
  setIssueStatus: (status: IssueStatus, issueId: string) => Observable<Issue>;
  setIssuesFixVersion(release: Release): Observable<Release>;
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

export interface GitStatus {
  files: string[];
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
  remoteRelease: RemoteRelease;
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

export interface FotingoArguments<T> extends Arguments {
  config: Config;
  data: { [P in keyof T]?: T[P] };
}

export interface ReleaseData {
  issues: string[];
  name: string;
  useDefaults: boolean;
}

export interface ReviewData {
  branch?: string;
  labels?: string[];
  reviewers?: string[];
  tracker: {
    enabled: boolean;
  };
  useDefaults: boolean;
}
