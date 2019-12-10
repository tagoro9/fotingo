/**
 * Branch tests
 */
import 'jest';
import { getIssueId, getName } from 'src/git/Branch';
import { GitConfig } from 'src/git/Config';
import data from '../lib/data';

const gitConfig: GitConfig = {
  baseBranch: 'master',
  branchTemplate: '{issue.key}.{issue.shortName}.{issue.sanitizedSummary}',
  remote: 'origin',
};

describe('Branch', () => {
  describe('getName', () => {
    test('replaces the placeholders with the issue data', () => {
      expect(getName(gitConfig, data.createIssue())).toMatchSnapshot();
    });

    test('ignores unknown template data', () => {
      expect(
        getName(
          {
            ...gitConfig,
            branchTemplate: '{does_not_exist}',
          },
          data.createIssue(),
        ),
      ).toMatchSnapshot();
    });
  });

  describe('getIssueId', () => {
    test('extract the issue id from the branch name using the template config', () => {
      expect(getIssueId(gitConfig, 'TEST-1234.b.my_issue')).toBe('TEST-1234');
      expect(
        getIssueId(
          {
            ...gitConfig,
            branchTemplate: '{issue.shortName}/{issue.key}_{issue.sanitizedSummary}',
          },
          'f/TEST-1234_my_project',
        ),
      ).toBe('TEST-1234');
      expect(
        getIssueId(
          {
            ...gitConfig,
            branchTemplate: '{issue.key}',
          },
          'TEST-1234',
        ),
      ).toBe('TEST-1234');
      expect(getIssueId(gitConfig, 'doesnotmatchtheregex')).toBe(undefined);
    });
  });
});
