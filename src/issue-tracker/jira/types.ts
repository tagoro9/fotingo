import { IssueType } from 'src/types';

export interface IssueTransition {
  id: number;
  name: string;
}

export interface JiraIssue {
  fields: {
    description?: string;
    issuetype: {
      name: string;
    };
    project: {
      id: string;
      key: string;
    };
    summary: string;
  };
  id: number;
  key: string;
  renderedFields: {
    description?: string;
  };
  transitions: IssueTransition[];
  url: string;
}

export interface JiraIssueStatus {
  description: string;
  id: string;
  name: string;
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
  key: string;
  name: string;
}

export interface JiraRelease {
  archived: boolean;
  description: string;
  id: string;
  name: string;
  project: Project;
  projectId: number;
  releaseDate: string;
  released: boolean;
  self: string;
  userReleaseDate: string;
}
