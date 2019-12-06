import {
  compose,
  converge,
  groupBy,
  head,
  join,
  lensProp,
  map as rMap,
  mapObjIndexed,
  prop,
  split,
  tail,
  toPairs,
  trim,
  unapply,
  view,
  zipObj,
} from 'ramda';
import { from, Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { editVirtualFile } from 'src/io/file-util';
import { Messenger } from 'src/io/messenger';
import { parseTemplate } from 'src/io/template-util';
import { Issue, IssueType, Release } from 'src/issue-tracker/Issue';
import { ReleaseConfig, ReleaseNotes } from 'src/types';

enum RELEASE_TEMPLATE_KEYS {
  VERSION = 'version',
  FIXED_ISSUES_BY_CATEGORY = 'fixedIssuesByCategory',
  FOTINGO_BANNER = 'fotingo.banner',
  JIRA_RELEASE = 'jira.release',
}

const ISSUE_TYPE_TO_RELEASE_SECTION: { [k in IssueType]: string } = {
  [IssueType.TASK]: 'Features',
  [IssueType.SUB_TASK]: 'Features',
  [IssueType.BUG]: 'Bug fixes',
  [IssueType.STORY]: 'Features',
  [IssueType.FEATURE]: 'Features',
};

/**
 * Allow the user to edit the initial release notes
 * @param initialReleaseNotes Initial release notes
 */
async function editReleaseNotes(initialNotes: string, messenger: Messenger): Promise<string> {
  messenger.inThread(true);
  const notes = await editVirtualFile({
    extension: 'md',
    initialContent: initialNotes,
    prefix: 'fotingo-review',
  });
  messenger.inThread(false);
  return notes;
}

function getReleaseNotesFromTemplate(data: Release, releaseConfig: ReleaseConfig): string {
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
        groupBy(compose(lens => view(lens, ISSUE_TYPE_TO_RELEASE_SECTION), lensProp, prop('type'))),
      )(data.issues),
      [RELEASE_TEMPLATE_KEYS.VERSION]: data.name,
      [RELEASE_TEMPLATE_KEYS.FOTINGO_BANNER]:
        '🚀 Release created with [fotingo](https://github.com/tagoro9/fotingo)',
      [RELEASE_TEMPLATE_KEYS.JIRA_RELEASE]: data.url || '',
    },
    template: releaseConfig.template,
  });
}

/**
 * Generate a function that generates the release notes
 * for a release
 * @param releaseConfig Release configuration
 * @param messenger Messenger
 */
export function getReleaseNotes(
  releaseConfig: ReleaseConfig,
  messenger: Messenger,
  release: Release,
  useDefaults: boolean,
): Observable<ReleaseNotes> {
  const initialReleaseNotes = getReleaseNotesFromTemplate(release, releaseConfig);
  return from(
    useDefaults ? [initialReleaseNotes] : editReleaseNotes(initialReleaseNotes, messenger),
  ).pipe(
    map(
      converge(unapply(zipObj(['title', 'body'])), [
        compose<string, string, string, string[], string>(head, split('\n'), trim),
        compose<string, string, string, string[], string[], string>(
          join('\n'),
          tail,
          split('\n'),
          trim,
        ),
      ]),
    ),
  );
}
