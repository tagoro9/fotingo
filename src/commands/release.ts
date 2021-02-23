import { flags } from '@oclif/command';
import { compose, filter, isNil, join, not } from 'ramda';
import { Observable, of } from 'rxjs';
import { map, switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { yes } from 'src/cli/flags';
import { FotingoCommand } from 'src/cli/FotingoCommand';
import { Emoji } from 'src/io/messenger';
import { getReleaseNotes } from 'src/templates/getReleaseNotes';
import {
  CreateRelease,
  JointRelease,
  LocalChanges,
  Release as JiraRelease,
  ReleaseData,
} from 'src/types';

export class Release extends FotingoCommand<JointRelease, ReleaseData> {
  static description = 'Create a release with your changes';

  static args = [
    {
      description: 'Name of the release to be created',
      name: 'release',
      required: true,
    },
  ];

  static flags = {
    issues: flags.string({
      char: 'i',
      description: 'Specify more issues to include in the release',
      multiple: true,
      required: false,
    }),
    simple: flags.boolean({
      char: 's',
      description: 'Do not use any issue tracker',
      name: 'simple',
    }),
    noVcsRelease: flags.boolean({
      char: 'n',
      description: 'Do not create a release in the remote VCS',
      name: 'no-vcs-release',
    }),
    yes,
  };

  /**
   * Given the local changes and a release data, build the object needed
   * to create a release in the issue tracker
   */
  private static buildReleaseData([data, { tracker, ...releaseData }]: [
    LocalChanges,
    ReleaseData,
  ]): CreateRelease {
    return {
      ...releaseData,
      ...data,
      submitRelease: tracker.enabled,
    };
  }

  protected getCommandData(): ReleaseData {
    const { args, flags } = this.parse(Release);
    return {
      issues: (flags.issues || []) as string[],
      name: args.release as string,
      tracker: {
        enabled: !flags.simple,
      },
      useDefaults: flags.yes as boolean,
      vcs: {
        enabled: !flags.noVcsRelease,
      },
    };
  }

  protected runCmd(commandData$: Observable<ReleaseData>): Observable<JointRelease> {
    const releaseInformation$ = commandData$.pipe(
      switchMap(this.git.getBranchInfo),
      withLatestFrom(commandData$),
      switchMap(this.getLocalChangesInformation),
      withLatestFrom(commandData$),
      map(Release.buildReleaseData),
    );

    const maybeCreateGithubRelease = (
      source: Observable<[JiraRelease, ReleaseData]>,
    ): Observable<JointRelease> => {
      return source.pipe(
        switchMap(([release, releaseData]) =>
          !releaseData.vcs.enabled
            ? of({ release })
            : getReleaseNotes(
                this.fotingo.release,
                this.messenger,
                release,
                releaseData.useDefaults,
              ).pipe(
                map((notes) => ({
                  notes,
                  release,
                })),
                switchMap(({ notes, release }) => this.github.createRelease(release, notes)),
              ),
        ),
      );
    };

    return releaseInformation$.pipe(
      tap((data) => this.messenger.emit(`Creating release ${data.name}`, Emoji.SHIP)),
      switchMap(this.tracker.createRelease),
      switchMap(this.tracker.setIssuesFixVersion),
      withLatestFrom(commandData$),
      maybeCreateGithubRelease,
      tap((data: JointRelease) =>
        this.messenger.emit(
          `Release created: ${compose(
            join('|'),
            filter(compose(not, isNil)),
          )([data.release.url, data.remoteRelease?.url])}`,
          Emoji.LINK,
        ),
      ),
    );
  }
}
