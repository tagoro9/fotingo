import { boundMethod } from 'autobind-decorator';
import {
  __,
  always,
  compose,
  converge,
  curry,
  evolve,
  groupBy,
  head,
  indexBy,
  join,
  length,
  Lens,
  lensProp,
  lt,
  map as rMap,
  mapObjIndexed,
  path,
  prop,
  replace,
  split,
  tail,
  take,
  toLower,
  toPairs,
  trim,
  unapply,
  uniq,
  view,
  when,
  zipObj,
} from 'ramda';
import { from, merge, Observable, of, throwError } from 'rxjs';
import { catchError, map, reduce, switchMap } from 'rxjs/operators';
import { editVirtualFile } from 'src/io/file-util';
import { Messenger } from 'src/io/messenger';
import { parseTemplate } from 'src/io/template-util';
import { HttpClient } from 'src/network/HttpClient';
import { HttpError } from 'src/network/HttpError';
import * as Turndown from 'turndown';

import { JiraConfig } from './Config';
import {
  CreateIssue,
  CreateRelease,
  Issue,
  IssueComment,
  IssueEditMeta,
  IssueStatus,
  IssueTransition,
  IssueType,
  IssueTypeData,
  JiraRelease,
  Project,
  Release,
  ReleaseNotes,
  User,
} from './Issue';
import { JiraErrorImpl } from './JiraError';
import { Tracker } from './Tracker';

const turnDownService = new Turndown();

const getShortName = (name: string) => {
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

enum RELEASE_TEMPLATE_KEYS {
  VERSION = 'version',
  FIXED_ISSUES_BY_CATEGORY = 'fixedIssuesByCategory',
  FOTINGO_BANNER = 'fotingo.banner',
}

const ISSUE_TYPE_TO_RELEASE_SECTION: { [k in IssueType]: string } = {
  [IssueType.TASK]: 'Features',
  [IssueType.SUB_TASK]: 'Features',
  [IssueType.BUG]: 'Bug fixes',
  [IssueType.STORY]: 'Features',
  [IssueType.FEATURE]: 'Features',
};

// Using Jira REST API v2 (https://developer.atlassian.com/cloud/jira/platform/rest/v2)
export class Jira implements Tracker {
  public setIssueStatus = curry(
    (status: IssueStatus, issueId: string): Observable<Issue> => {
      return this.getIssue(issueId).pipe(
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
        catchError(this.mapError),
      );
    },
  );

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

  public getCurrentUser(): Observable<User> {
    return this.client
      .get<User>('/myself', {
        qs: { expand: 'groups' },
      })
      .pipe(
        map(prop('body')),
        catchError(this.mapError),
      );
  }

  @boundMethod
  public getIssue(id: string): Observable<Issue> {
    return this.client
      .get<Issue>(`/issue/${id}`, {
        qs: { expand: 'transitions, renderedFields' },
      })
      .pipe(
        map(prop('body')),
        map(issue => ({
          ...issue,
          fields: {
            ...issue.fields,
            description: issue.renderedFields.description
              ? turnDownService.turndown(issue.renderedFields.description)
              : undefined,
          },
          sanitizedSummary: compose<string, string, string, string, string, string, string>(
            take(72),
            replace(/(_|-)$/, ''),
            replace(/\s|\(|\)|__+/g, '_'),
            replace(/\/|\.|--=/g, '-'),
            replace(/,|\[|]|"|'|”|“|@|’|`|:|\$|\?|\*|<|>|&|~/g, ''),
            toLower,
          )(issue.fields.summary),
          type: issue.fields.issuetype.name as IssueType,
          url: `${this.config.root}/browse/${issue.key}`,
        })),
        catchError(this.mapError),
      );
  }

  public isValidIssueName(name: string): boolean {
    return /\w+-\d+/i.test(name);
  }

  public getProject(id: string): Observable<Project> {
    return this.client.get<Project>(`/project/${id}`).pipe(
      map(
        compose(
          (data: Project) =>
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
                ),
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
          .pipe(
            map(prop('body')),
            catchError(this.mapError),
          ),
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

  public getIssueEditMeta(issueId: string): Observable<IssueEditMeta> {
    return this.client.get<IssueEditMeta>(`/issue/${issueId}/editmeta`).pipe(
      map(prop('body')),
      catchError(this.mapError),
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
          compose(
            lt(1),
            length,
            uniq,
            rMap(path(['fields', 'project', 'id'])),
            prop('issues'),
          ),
          () => {
            throw new Error('Issues in multiple projects');
          },
        ),
      ),
      switchMap(this.createReleaseNotes),
      map(
        converge(unapply(zipObj(['title', 'body'])), [
          compose<string, string, string, string[], string>(
            head,
            split('\n'),
            trim,
          ),
          compose<string, string, string, string[], string[], string>(
            join('\n'),
            tail,
            split('\n'),
            trim,
          ),
        ]),
      ),
      switchMap(notes => {
        if (data.submitRelease) {
          return this.createVersion(data, notes).pipe(
            map(release => ({
              id: release.id,
              issues: data.issues,
              name: release.name,
              notes,
              url: `${this.config.root}/projects/${release.project.key}/versions/${release.id}`,
            })),
          );
        }
        return of({
          id: data.name,
          issues: data.issues,
          name: data.name,
          notes,
        });
      }),
    );
  }

  @boundMethod
  public setIssuesFixVersion(release: Release): Observable<Release> {
    return from(release.issues)
      .pipe(
        switchMap(issue =>
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
            reduce<Issue>((acc, val) => acc.concat(val), []),
            map(issues => head(issues)),
          ),
        ),
      )
      .pipe(
        reduce<Issue>((acc, val) => acc.concat(val), []),
        map(always(release)),
      );
  }

  @boundMethod
  private createReleaseNotes(data: CreateRelease): Observable<string> {
    const initialReleaseNotes = this.getReleaseNotesFromTemplate(data);
    return from(
      data.useDefaults ? [initialReleaseNotes] : this.editReleaseNotes(initialReleaseNotes),
    );
  }

  private async editReleaseNotes(initialReleaseNotes: string): Promise<string> {
    this.messenger.inThread(true);
    const notes = await editVirtualFile({
      extension: 'md',
      initialContent: initialReleaseNotes,
      prefix: 'fotingo-review',
    });
    this.messenger.inThread(false);
    return notes;
  }

  private getReleaseNotesFromTemplate(data: CreateRelease): string {
    return parseTemplate<RELEASE_TEMPLATE_KEYS>({
      data: {
        [RELEASE_TEMPLATE_KEYS.FIXED_ISSUES_BY_CATEGORY]: compose(
          join('\n'),
          rMap(([title, list]) => `# ${title}:\n\n${list}`),
          toPairs,
          mapObjIndexed(
            compose(
              join('\n'),
              rMap((issue: Issue) => `* [#${issue.key}](${issue.url}): ${issue.fields.summary}`),
            ),
          ),
          groupBy(
            compose(
              view((__ as unknown) as Lens, ISSUE_TYPE_TO_RELEASE_SECTION),
              lensProp,
              prop('type'),
            ),
          ),
        )(data.issues),
        [RELEASE_TEMPLATE_KEYS.VERSION]: data.name,
        [RELEASE_TEMPLATE_KEYS.FOTINGO_BANNER]:
          '🚀 Release created with [fotingo](https://github.com/tagoro9/fotingo)',
      },
      template: this.config.releaseTemplate,
    });
  }

  private createVersion(data: CreateRelease, releaseNotes: ReleaseNotes): Observable<JiraRelease> {
    return this.client
      .post<JiraRelease>(`/version`, {
        body: {
          archived: false,
          description: releaseNotes.title,
          name: data.name,
          projectId: head(data.issues.map(path(['fields', 'project', 'id']))),
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

  private getTransitionForStatus(issue: Issue, status: IssueStatus): IssueTransition | undefined {
    return issue.transitions.find(transition => statusRegex[status].test(transition.name));
  }

  private mapError(e: NodeJS.ErrnoException | HttpError) {
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
