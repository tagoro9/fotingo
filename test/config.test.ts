/**
 * Config testsC
 */
import * as cosmiconfig from 'cosmiconfig';
import { readConfig } from 'src/config';
import { data } from 'test/lib/data';
import { describe, expect, test, vi } from 'vitest';

vi.mock('cosmiconfig', () => ({
  cosmiconfigSync: vi.fn().mockReturnValue(vi.fn()),
}));

const mockCosmiconfig = vi.mocked(cosmiconfig);

// TODO Use real Config instances for tests

const mockSearch = (search: () => unknown) => {
  mockCosmiconfig.cosmiconfigSync.mockImplementation(
    () =>
      ({
        search,
      } as unknown as ReturnType<typeof mockCosmiconfig.cosmiconfigSync>),
  );
};

describe('config', () => {
  test('reads config from current path and home path', () => {
    const search = vi.fn().mockReturnValue({ isEmpty: true });
    mockSearch(search);
    expect(readConfig()).toEqual({});
    expect(search).toHaveBeenCalledTimes(2);
    expect(search).toBeCalledWith(process.env.HOME);
  });

  test('merges configuration objects', () => {
    const search = vi.fn().mockReturnValueOnce({
      config: {
        someDeepKey: { key1: 'value', keyShared: 'first' },
        someKey: 'value',
        test: 'value',
      },
      isEmpty: false,
    });
    search.mockReturnValueOnce({
      config: {
        anotherValue: 'anotherValue',
        someDeepKey: { key2: 'value', keyShared: 'second' },
        test: 'value2',
      },
      isEmpty: false,
    });
    mockSearch(search);
    expect(readConfig()).toEqual({
      anotherValue: 'anotherValue',
      someDeepKey: {
        key1: 'value',
        key2: 'value',
        keyShared: 'first',
      },
      someKey: 'value',
      test: 'value',
    });
  });

  test('reads config from env variables', () => {
    const search = vi.fn().mockReturnValueOnce({
      config: {
        jira: data.createTrackerConfig(),
        github: data.createRemoteConfig(),
      },
      isEmpty: false,
    });
    mockSearch(search);
    process.env.GITHUB_TOKEN = 'github-token';
    process.env.FOTINGO_JIRA_ROOT = 'https://test.com';
    process.env.FOTINGO_JIRA_USER_LOGIN = 'test@test.com';
    process.env.FOTINGO_JIRA_USER_TOKEN = 'jira-token';
    expect(readConfig()).toMatchInlineSnapshot(`
      {
        "github": {
          "authToken": "github-token",
          "owner": "tagoro9",
          "pullRequestTemplate": "{summary}",
          "repo": "tagoro9/fotingo",
        },
        "jira": {
          "root": "https://test.com",
          "status": {
            "BACKLOG": /backlog/i,
            "DONE": /done/i,
            "IN_PROGRESS": /progress/i,
            "IN_REVIEW": /review/i,
            "SELECTED_FOR_DEVELOPMENT": /to do/i,
          },
          "user": {
            "login": "test@test.com",
            "token": "jira-token",
          },
        },
      }
    `);
  });
});
