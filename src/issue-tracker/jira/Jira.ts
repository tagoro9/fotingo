import { boundMethod } from 'autobind-decorator';
import {
  always,
  compose,
  evolve,
  head,
  indexBy,
  length,
  lt,
  map as rMap,
  path,
  prop,
  replace,
  take,
  toLower,
  uniq,
  when,
} from 'ramda';
import { from, merge, Observable, of, throwError } from 'rxjs';
import { catchError, map, mergeMap, reduce, switchMap } from 'rxjs/operators';
import { Messenger } from 'src/io/messenger';
import { Tracker } from 'src/issue-tracker/Tracker';
import { HttpClient } from 'src/network/HttpClient';
import { HttpError } from 'src/network/HttpError';
import {
  CreateIssue,
  CreateRelease,
  Issue,
  IssueComment,
  IssueStatus,
  IssueType,
  Release,
  User,
} from 'src/types';
import * as Turndown from 'turndown';

import { JiraConfig } from './Config';
import { JiraErrorImpl } from './JiraError';
import {
  IssueTransition,
  IssueTypeData,
  JiraIssue,
  JiraRelease,
  Project,
  RawProject,
} from './types';

const turnDownService = new Turndown();

const getShortName = (name: string): string => {
  if (name.match(/feature|story/i)) {
    return 'f';
  }
  if (name.match(/task/i)) {
    return 'c';
  }
  return name[0].toLowerCase();
};

// TODO Make this part of config
const statusRegex = {
  [IssueStatus.BACKLOG]: /backlog/i,
  [IssueStatus.IN_PROGRESS]: /in progress/i,
  [IssueStatus.IN_REVIEW]: /review/i,
  [IssueStatus.DONE]: /done/i,
  [IssueStatus.SELECTED_FOR_DEVELOPMENT]: /(todo)|(to do)|(selected for development)/i,
};

// Using Jira REST API v2 (https://developer.atlassian.com/cloud/jira/platform/rest/v2)
export class Jira implements Tracker {
  private client: HttpClient;
  private config: JiraConfig;
  private messenger: Messenger;

  constructor(config: JiraConfig, messenger: Messenger) {
    this.config = config;
    this.messenger = messenger;
    // TODO Add a new client for agile API https://developer.atlassian.com/cloud/jira/software/rest/
    this.client = new HttpClient({
      allowConcurrentRequests: false,
      auth: { pass: config.user.token, user: config.user.login },
      root: `${config.root}/rest/api/2`,
    });
  }

  public setIssueStatus(status: IssueStatus, issueId: string): Observable<Issue> {
    return this.getJiraIssue(issueId).pipe(
      switchMap(issue => {
        const transition = this.getTransitionForStatus(issue, status);
        if (!transition) {
          throw new Error('Missing status');
        }
        return this.client
          .post<Issue>(`/issue/${issue.key}/transitions`, {
            body: {
              transition: transition.id,
            },
          })
          .pipe(map(always(issue)));
      }),
      map(this.convertIssue),
      catchError(this.mapError),
    );
  }

  public getCurrentUser(): Observable<User> {
    return this.client
      .get<User>('/myself', {
        qs: { expand: 'groups' },
      })
      .pipe(map(prop('body')), catchError(this.mapError));
  }

  @boundMethod
  public getIssue(id: string): Observable<Issue> {
    return this.getJiraIssue(id).pipe(map(this.convertIssue));
  }

  public isValidIssueName(name: string): boolean {
    return /\w+-\d+/i.test(name);
  }

  public getProject(id: string): Observable<Project> {
    return this.client.get<RawProject>(`/project/${id}`).pipe(
      map(
        compose(
          (data: RawProject) =>
            evolve(
              {
                issueTypes: compose(
                  indexBy<IssueTypeData>(prop('name')),
                  rMap(
                    (typeData: { id: number; name: string }): IssueTypeData => ({
                      ...typeData,
                      shortName: getShortName(typeData.name),
                    }),
                  ),
                ) as (d: RawProject['issueTypes']) => Project['issueTypes'],
              },
              data,
            ),
          prop('body'),
        ),
      ),
      catchError(this.mapError),
    );
  }

  @boundMethod
  public createIssueForCurrentUser(data: CreateIssue): Observable<Issue> {
    return this.getCurrentUser().pipe(switchMap(user => this.createIssue(data, user)));
  }

  @boundMethod
  public createIssue(data: CreateIssue, user: User): Observable<Issue> {
    return this.getProject(data.project).pipe(
      switchMap(project =>
        this.client
          .post<Issue>('/issue', {
            body: {
              fields: {
                assignee: {
                  name: user.key,
                },
                description: data.description,
                issuetype: {
                  id: project.issueTypes[data.type].id,
                },
                // TODO Make sure labels exist
                labels: data.labels,
                project: {
                  id: project.id,
                },
                summary: data.title,
              },
            },
          })
          .pipe(map(prop('body')), catchError(this.mapError)),
      ),
    );
  }

  public addLabelToIssue(issueId: string, label: string): Observable<Issue> {
    this.messenger.emit('This is a test so TS compiles');
    return this.client
      .put<void>(`/issue/${issueId}/`, {
        body: {
          update: {
            labels: [{ add: label }],
          },
        },
      })
      .pipe(
        catchError(this.mapError),
        switchMap(() => this.getIssue(issueId)),
      );
  }

  public addCommentToIssue(issueId: string, comment: string): Observable<IssueComment> {
    return this.getIssue(issueId).pipe(
      switchMap(issue =>
        this.client.post<IssueComment>(`/issue/${issue.key}/comment`, {
          body: { body: comment },
        }),
      ),
      map(prop('body')),
      catchError(this.mapError),
    );
  }

  @boundMethod
  public createRelease(data: CreateRelease): Observable<Release> {
    return of(data).pipe(
      map(
        when(
          compose(lt(1), length, uniq, rMap(path(['fields', 'project', 'id'])), prop('issues')),
          () => {
            throw new Error('Issues in multiple projects');
          },
        ),
      ),
      switchMap(() => {
        if (data.submitRelease) {
          return this.createVersion(data).pipe(
            map(release => ({
              id: release.id,
              issues: data.issues,
              name: release.name,
              url: `${this.config.root}/projects/${release.project.key}/versions/${release.id}`,
            })),
          );
        }
        return of({
          id: data.name,
          issues: data.issues,
          name: data.name,
        });
      }),
    );
  }

  @boundMethod
  public setIssuesFixVersion(release: Release): Observable<Release> {
    return from(release.issues)
      .pipe(
        mergeMap(issue =>
          merge(
            this.setIssueStatus(IssueStatus.DONE, issue.key),
            this.client
              .put<Issue>(`/issue/${issue.key}`, {
                body: {
                  update: {
                    fixVersions: [{ add: { name: release.name } }],
                  },
                },
              })
              .pipe(map(prop('body'))),
          ).pipe(
            reduce<Issue, Issue[]>((acc, val) => acc.concat(val), []),
            map(issues => head(issues)),
          ),
        ),
      )
      .pipe(
        reduce<Issue, Issue[]>((acc, val) => acc.concat(val), []),
        map(always(release)),
      );
  }

  @boundMethod
  private getJiraIssue(id: string): Observable<JiraIssue> {
    return this.client
      .get<JiraIssue>(`/issue/${id}`, {
        qs: { expand: 'transitions, renderedFields' },
      })
      .pipe(map(prop('body')), catchError(this.mapError));
  }

  @boundMethod
  private convertIssue(issue: JiraIssue): Issue {
    return {
      description: issue.renderedFields.description
        ? turnDownService.turndown(issue.renderedFields.description)
        : undefined,
      id: issue.id,
      key: issue.key,
      project: issue.fields.project.id,
      sanitizedSummary: compose<string, string, string, string, string, string, string>(
        take(72),
        replace(/(_|-)$/, ''),
        replace(/\s|\(|\)|__+/g, '_'),
        replace(/\/|\.|--=/g, '-'),
        replace(/,|\[|]|"|'|”|“|@|’|`|:|\$|\?|\*|<|>|&|~|‘/g, ''),
        toLower,
      )(issue.fields.summary),
      summary: issue.fields.summary,
      type: issue.fields.issuetype.name as IssueType,
      url: `${this.config.root}/browse/${issue.key}`,
    };
  }

  private createVersion(data: CreateRelease): Observable<JiraRelease> {
    return this.client
      .post<JiraRelease>(`/version`, {
        body: {
          archived: false,
          description: data.name,
          name: data.name,
          projectId: head(data.issues.map(path(['project']))),
          releaseDate: new Date().toISOString().substring(0, 10),
          released: true,
        },
      })
      .pipe(
        map(prop('body')),
        switchMap(release =>
          this.getProject(String(release.projectId)).pipe(
            map(project => ({ ...release, project })),
          ),
        ),
      );
  }

  private getTransitionForStatus(
    issue: JiraIssue,
    status: IssueStatus,
  ): IssueTransition | undefined {
    return issue.transitions.find(transition => statusRegex[status].test(transition.name));
  }

  private mapError(e: NodeJS.ErrnoException | HttpError): Observable<never> {
    if ('status' in e) {
      const code = e.status;
      const message =
        (e.body.errorMessages && e.body.errorMessages[0]) ||
        'Something went wrong when connecting with jira';
      return throwError(new JiraErrorImpl(message, code));
    } else {
      return throwError(e);
    }
  }
}
