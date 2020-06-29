import { Observable } from 'rxjs';
import {
  CreateIssue,
  CreateRelease,
  Issue,
  IssueComment,
  IssueStatus,
  Release,
  User,
} from 'src/types';

export interface Tracker {
  addCommentToIssue: (issueId: string, comment: string) => Observable<IssueComment>;
  addLabelToIssue: (issueId: string, label: string) => Observable<Issue>;
  createIssue: (data: CreateIssue, user: User) => Observable<Issue>;
  createIssueForCurrentUser: (data: CreateIssue) => Observable<Issue>;
  createRelease: (data: CreateRelease) => Observable<Release>;
  getCurrentUser: () => Observable<User>;
  getCurrentUserOpenIssues: () => Observable<Issue[]>;
  getIssue: (issueId: string) => Observable<Issue>;
  isValidIssueName: (name: string) => boolean;
  readonly name: string;
  setIssueStatus: (status: IssueStatus, issueId: string) => Observable<Issue>;
  setIssuesFixVersion(release: Release): Observable<Release>;
}
