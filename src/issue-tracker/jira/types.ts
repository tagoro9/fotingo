import { IssueType } from 'src/types';

export interface IssueTransition {
  id: number;
  name: string;
}

export interface JiraIssue {
  id: number;
  key: string;
  sanitizedSummary: string;
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
  transitions: IssueTransition[];
  url: string;
}

export interface IssueTypeData {
  id: number;
  name: string;
  shortName: string;
}

export interface Project {
  id: number;
  issueTypes: { [type in IssueType]: IssueTypeData };
  key: string;
  name: string;
}

export interface RawProject {
  id: number;
  issueTypes: Array<{ id: number; name: string }>;
  name: string;
  key: string;
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
