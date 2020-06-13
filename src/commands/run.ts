/**
 * Command runner. Given the input from yargs, run the appropriate command
 * if it is a valid command. Show the help otherwise
 */

import { existsSync, mkdirSync } from 'fs';
import { homedir } from 'os';
import * as R from 'ramda';
import { from, Observable } from 'rxjs';
import { concatMap, map, reduce, switchMap } from 'rxjs/operators';
import { FotingoArguments } from 'src/commands/FotingoArguments';
import { requiredConfigs, write } from 'src/config';
import { enhanceConfigWithRuntimeArgs } from 'src/enhanceConfig';
import { Messenger } from 'src/io/messenger';
import { Config } from 'src/types';
import { renderUi } from 'src/ui/ui';
import { showHelp } from 'yargs';

// TODO I think this is not needed anymore if using yargs handlers
export enum CommandName {
  RELEASE = 'release',
  REVIEW = 'review',
  START = 'start',
}

const messenger = new Messenger();

/**
 * Ask the user for the required configurations that are currently missing and write them to the closest config
 * file
 * @param msg Messenger
 * @param args Args
 */
const askForConfigs = (msg: Messenger, args: FotingoArguments): Observable<FotingoArguments> =>
  from(requiredConfigs.filter((cfg) => R.path(cfg.path, args.config) === undefined)).pipe(
    concatMap((requiredConfig) =>
      msg.request(requiredConfig.request).pipe(map((value) => [requiredConfig.path, value])),
    ),
    reduce<[string[], string], Partial<Config>>((acc, val) => {
      return R.set(R.lensPath(val[0]), val[1], acc);
    }, {}),
    map(write),
    map((data) => ({ config: data })),
    map((d) => R.mergeDeepRight(args, d) as FotingoArguments),
  );

/**
 * Create the fotingo config folder if it doesn't exist
 */
const createConfigFolder: () => void = () => {
  const path = `${homedir()}/.fotingo_config`;
  if (!existsSync(path)) {
    mkdirSync(path);
  }
};

export const run: (args: FotingoArguments) => void = R.ifElse(
  R.compose(R.contains(R.__, Object.values(CommandName)), R.head, R.prop('_')),
  R.compose(
    R.converge(
      (
        cmd: (args: FotingoArguments, messenger: Messenger) => Observable<unknown>,
        args: FotingoArguments,
      ) => {
        renderUi({
          args,
          cmd: () =>
            askForConfigs(messenger, args).pipe(
              switchMap((augmentedArgs) => cmd(augmentedArgs, messenger)),
            ),
          isDebugging: process.env.DEBUG !== undefined,
          messenger,
        });
      },
      [
        R.compose(
          // eslint-disable-next-line @typescript-eslint/no-var-requires
          (path: string) => require(path).cmd,
          R.concat('./'),
          R.replace(/^./, R.toUpper),
          R.head,
          R.prop('_'),
        ),
        (a: FotingoArguments): FotingoArguments => ({
          ...a,
          config: enhanceConfigWithRuntimeArgs(a.config, a),
        }),
        (): void => createConfigFolder(),
      ],
    ),
  ),
  () => showHelp(),
);
