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
  getIssue: (issueId: string) => Observable<Issue>;
  createIssue: (data: CreateIssue, user: User) => Observable<Issue>;
  createIssueForCurrentUser: (data: CreateIssue) => Observable<Issue>;
  getCurrentUser: () => Observable<User>;
  setIssueStatus: (status: IssueStatus, issueId: string) => Observable<Issue>;
  addCommentToIssue: (issueId: string, comment: string) => Observable<IssueComment>;
  addLabelToIssue: (issueId: string, label: string) => Observable<Issue>;
  createRelease: (data: CreateRelease) => Observable<Release>;
  isValidIssueName: (name: string) => boolean;
  setIssuesFixVersion(release: Release): Observable<Release>;
}
