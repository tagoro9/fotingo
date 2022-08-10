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
import { from, merge, Observable, of, throwError, zip } from 'rxjs';
import { catchError, map, mergeMap, reduce, switchMap, withLatestFrom } from 'rxjs/operators';
import { Messenger } from 'src/io/messenger';
import { Tracker } from 'src/issue-tracker/Tracker';
import { HttpClient } from 'src/network/HttpClient';
import { HttpError } from 'src/network/HttpError';
import {
  CreateIssue,
  CreateRelease,
  CreateStandaloneIssue,
  CreateSubTask,
  Issue,
  IssueComment,
  IssueStatus,
  IssueType,
  Release,
  TrackerConfig,
  User,
} from 'src/types';
import Turndown from 'turndown';

import { JiraErrorImpl } from './JiraError';
import {
  IssueTransition,
  IssueTypeData,
  JiraIssue,
  JiraIssueStatus,
  JiraRelease,
  Project,
  RawProject,
} from './types';

const turnDownService = new Turndown();

type LightIssue = Pick<Issue, 'id' | 'key'>;

const getShortName = (name: string): string => {
  if (/feature|story/i.test(name)) {
    return 'f';
  }
  if (/task/i.test(name)) {
    return 'c';
  }
  return name[0].toLowerCase();
};

const isCreateSubTask = (
  createIssue: CreateSubTask | CreateStandaloneIssue,
): createIssue is CreateSubTask => {
  return 'parent' in createIssue;
};

// Using Jira REST API v2 (https://developer.atlassian.com/cloud/jira/platform/rest/v2)
export class Jira implements Tracker {
  private client: HttpClient;
  private config: TrackerConfig;
  private messenger: Messenger;
  public readonly name = 'Jira';

  constructor(config: TrackerConfig, messenger: Messenger) {
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
      switchMap((issue) => {
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

  @boundMethod
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
    return this.getCurrentUser().pipe(
      switchMap((user) => this.createIssue(data, user)),
      switchMap((issue) => this.getIssue(issue.key)),
    );
  }

  @boundMethod
  private createIssue(data: CreateIssue, user: User): Observable<LightIssue> {
    const project$ = (
      isCreateSubTask(data)
        ? this.getIssue(data.parent).pipe(map((parent) => parent.project.key))
        : of(data.project)
    ).pipe(switchMap(this.getProject));
    const parent$ = isCreateSubTask(data) ? this.getIssue(data.parent) : of(undefined);
    return project$.pipe(
      withLatestFrom(parent$),
      switchMap(([project, parent]: [Project, Issue | undefined]) => {
        return this.client
          .post<LightIssue>('/issue', {
            body: {
              fields: {
                assignee: {
                  name: user.accountId,
                },
                description: data.description,
                issuetype: {
                  id: project.issueTypes[data.type].id,
                },
                // TODO Make sure labels exist
                labels: data.labels,
                ...(parent && {
                  parent: {
                    key: parent.key,
                  },
                }),
                project: {
                  id: project.id,
                },
                summary: data.title,
              },
            },
          })
          .pipe(map(prop('body')), catchError(this.mapError));
      }),
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
      switchMap((issue) =>
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
            map((release) => ({
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
        mergeMap((issue) =>
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
            reduce<Issue, Issue[]>((accumulator, value) => [...accumulator, value], []),
            map((issues) => head(issues)),
          ),
        ),
      )
      .pipe(
        reduce<Issue, Issue[]>((accumulator, value) => [...accumulator, value], []),
        map(always(release)),
      );
  }

  public getCurrentUserOpenIssues(): Observable<Issue[]> {
    return zip(this.getCurrentUser(), this.getStatuses()).pipe(
      switchMap(([user, status]: [User, Partial<Record<IssueStatus, JiraIssueStatus>>]) => {
        return this.client.get<{ issues: JiraIssue[] }>(`/search`, {
          qs: {
            expand: 'transitions, renderedFields',
            jql: `assignee=${user.accountId} AND status IN (${[
              IssueStatus.BACKLOG,
              IssueStatus.SELECTED_FOR_DEVELOPMENT,
            ]
              // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
              .map((s) => (status[s] ? "'" + status[s]!.name + "'" : undefined))
              .filter((s) => s !== undefined)
              .join(',')}) ORDER BY CREATED DESC`,
          },
        });
      }),
      map(prop('body')),
      map((data) => {
        return rMap(this.convertIssue, data.issues).filter(
          (index) => index.type.toString() !== 'Epic',
        );
      }),
      catchError(this.mapError),
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
      project: {
        id: issue.fields.project.id,
        key: issue.fields.project.key,
      },
      sanitizedSummary: compose<string, string, string, string, string, string, string>(
        take(72),
        replace(/([_-])$/, ''),
        replace(/\s|\(|\)|__+/g, '_'),
        replace(/\/|\.|--=/g, '-'),
        replace(/["$&'*,:;<>?@[\]`~‘’“”]/g, ''),
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
          releaseDate: new Date().toISOString().slice(0, 10),
          released: true,
        },
      })
      .pipe(
        map(prop('body')),
        switchMap((release) =>
          this.getProject(String(release.projectId)).pipe(
            map((project) => ({ ...release, project })),
          ),
        ),
      );
  }

  // TODO This should be cacheable, but cacheable does not understand observables yer
  private getStatuses(): Observable<Partial<Record<IssueStatus, JiraIssueStatus>>> {
    return this.client.get<JiraIssueStatus[]>('/status').pipe(
      map(prop('body')),
      map((status: JiraIssueStatus[]) => {
        return Object.fromEntries(
          Object.entries(IssueStatus).map(([key, value]) => [
            key,
            status.find((t) => this.config.status[value].test(t.name)),
          ]),
        );
      }),
    );
  }

  private getTransitionForStatus(
    issue: JiraIssue,
    status: IssueStatus,
  ): IssueTransition | undefined {
    return issue.transitions.find((transition) => this.config.status[status].test(transition.name));
  }

  private mapError(error: NodeJS.ErrnoException | HttpError): Observable<never> {
    if ('status' in error) {
      const code = error.status;
      if (code === 401) {
        return throwError(
          new JiraErrorImpl(
            'Could not authenticate with Jira. Double check that your credentials are correct',
            code,
          ),
        );
      }
      const message =
        (error.body.errorMessages && error.body.errorMessages[0]) ||
        'Something went wrong when connecting with jira';
      return throwError(new JiraErrorImpl(message, code));
    } else {
      return throwError(error);
    }
  }
}
