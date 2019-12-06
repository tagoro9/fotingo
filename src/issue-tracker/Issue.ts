/**
 * Issue
 */

export enum IssueType {
  BUG = 'Bug',
  FEATURE = 'Feature',
  STORY = 'Story',
  SUB_TASK = 'Sub-task',
  TASK = 'Task',
}

export interface IssueEditMeta {
  fields: object;
}

export interface IssueCommentAuthor {
  name: string;
  displayName: string;
  active: boolean;
}

export interface IssueComment {
  id: string;
  author: IssueCommentAuthor;
  updateAuthor: IssueCommentAuthor;
  body: string;
  updated: string;
  created: string;
}

export interface IssueTypeData {
  id: number;
  name: string;
  shortName: string;
}

export enum IssueStatus {
  BACKLOG = 'BACKLOG',
  DONE = 'DONE',
  IN_PROGRESS = 'IN_PROGRESS',
  IN_REVIEW = 'IN_REVIEW',
  SELECTED_FOR_DEVELOPMENT = 'SELECTED_FOR_DEVELOPMENT',
}

export interface IssueTransition {
  id: number;
  name: string;
}

// TODO Decouple this from the jira specific fields. There shoudl be a JiraIssue interface and a mapper from that one to this one
export interface Issue {
  id: number;
  key: string;
  sanitizedSummary: string;
  type: IssueType;
  transitions: IssueTransition[];
  renderedFields: {
    description?: string;
  };
  fields: {
    summary: string;
    description?: string;
    issuetype: {
      name: string;
    };
    project: {
      id: string;
    };
  };
  url: string;
}

export interface CreateIssue {
  description?: string;
  labels?: string[];
  project: string;
  title: string;
  type: IssueType;
}

export interface CreateRelease {
  name: string;
  issues: Issue[];
  submitRelease: boolean;
  useDefaults: boolean;
}

export interface GetIssue {
  id: string;
}

export interface User {
  key: string;
  groups: {
    items: {
      name: string;
    };
  };
}

export interface Project {
  id: number;
  issueTypes: { [type in IssueType]: IssueTypeData };
  name: string;
  key: string;
}

export interface JiraProject {
  id: number;
  issueTypes: Array<{ id: number; name: string }>;
  name: string;
  key: string;
}

export interface ReleaseNotes {
  title: string;
  body: string;
}

export interface Release {
  id: string;
  name: string;
  issues: Issue[];
  url?: string;
}

export interface JiraRelease {
  self: string;
  id: string;
  description: string;
  name: string;
  archived: boolean;
  released: boolean;
  releaseDate: string;
  userReleaseDate: string;
  project: Project;
  projectId: number;
}
