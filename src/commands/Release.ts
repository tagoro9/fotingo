import { Observable, of } from 'rxjs';
import { map, switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { Git } from 'src/git/Git';
import { Github } from 'src/git/Github';
import { JointRelease, Remote } from 'src/git/Remote';
import { Emoji, Messenger } from 'src/io/messenger';
import { CreateRelease } from 'src/issue-tracker/Issue';
import { Jira } from 'src/issue-tracker/Jira';
import { Tracker } from 'src/issue-tracker/Tracker';
import { getReleaseNotes } from 'src/templates/getReleaseNotes';

import { FotingoArguments } from './FotingoArguments';
import { getLocalChangesInformation, LocalChanges } from './util';

interface ReleaseData {
  name: string;
  issues: string[];
  useDefaults: boolean;
  tracker: {
    enabled: boolean;
  };
}

const buildReleaseData = ([data, { issues, tracker, ...releaseData }]: [
  LocalChanges,
  ReleaseData,
]): CreateRelease => ({
  ...data,
  ...releaseData,
  submitRelease: tracker.enabled,
});

const getCommandData = (args: FotingoArguments): ReleaseData => {
  return {
    issues: (args.issue || []) as string[],
    name: args.releaseName as string,
    tracker: {
      enabled: !args.simple,
    },
    useDefaults: args.yes as boolean,
  };
};

export const cmd = (args: FotingoArguments, messenger: Messenger): Observable<JointRelease> => {
  const git: Git = new Git(args.config.git, messenger);
  const jira: Tracker = new Jira(args.config.jira, messenger);
  const github: Remote = new Github(args.config.github, messenger, git);
  const commandData$ = of(args).pipe(map(getCommandData));

  const releaseInformation$ = commandData$.pipe(
    switchMap(git.getBranchInfo),
    withLatestFrom(commandData$),
    switchMap(getLocalChangesInformation(jira, messenger)),
    withLatestFrom(commandData$),
    map(buildReleaseData),
  );

  return releaseInformation$.pipe(
    tap(data => messenger.emit(`Creating release ${data.name}`, Emoji.SHIP)),
    switchMap(jira.createRelease),
    switchMap(jira.setIssuesFixVersion),
    withLatestFrom(commandData$),
    switchMap(([release, { useDefaults }]) =>
      getReleaseNotes(args.config.release, messenger, release, useDefaults).pipe(
        map(notes => ({
          notes,
          release,
        })),
      ),
    ),
    switchMap(({ notes, release }) => github.createRelease(release, notes)),
    tap((data: JointRelease) =>
      messenger.emit(
        `Release created: ${data.release.url} | ${data.remoteRelease.url}`,
        Emoji.LINK,
      ),
    ),
  );
};
