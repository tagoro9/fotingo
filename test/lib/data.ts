import * as faker from 'faker';
import { GitConfig } from 'src/git/Config';
import { JiraIssue } from 'src/issue-tracker/jira/types';
import { HttpResponse } from 'src/network/HttpClient';
import {
  Config,
  Issue,
  IssueStatus,
  IssueType,
  Release,
  ReleaseConfig,
  TrackerConfig,
  User,
} from 'src/types';

/**
 * Data factory used to generate mock data for the tests
 */
export const data = {
  createJiraIssue(overrides: { summary?: string } = {}): JiraIssue {
    const defaultSummary = faker.name.jobDescriptor();
    const issueType = faker.random.arrayElement(Object.values(IssueType));
    return {
      fields: {
        description: faker.lorem.paragraph(),
        issuetype: {
          name: issueType,
        },
        project: {
          id: faker.lorem.word(),
          key: faker.lorem.word(),
        },
        summary: overrides.summary || defaultSummary,
      },
      id: faker.datatype.number(5000),
      key: `FOTINGO-${faker.datatype.number(5000)}`,
      renderedFields: {
        description: undefined,
      },
      transitions: [],
      url: faker.internet.url(),
    };
  },
  createIssue(type?: IssueType): Issue {
    const summary = faker.name.jobDescriptor();
    const sanitizedSummary = faker.helpers.slugify(summary);
    const issueType = type || faker.random.arrayElement(Object.values(IssueType));
    return {
      description: faker.lorem.paragraph(),
      id: faker.datatype.number(5000),
      key: `FOTINGO-${faker.datatype.number(5000)}`,
      project: {
        id: faker.lorem.word(),
        key: faker.lorem.word(),
      },
      sanitizedSummary,
      summary,
      type: issueType,
      url: faker.internet.url(),
    };
  },
  createGitConfig(): GitConfig {
    return {
      baseBranch: 'master',
      branchTemplate: '{issue.key}',
      remote: 'origin',
    };
  },
  createRemoteConfig(): Config['github'] {
    return {
      authToken: faker.random.alphaNumeric(10),
      owner: 'tagoro9',
      pullRequestTemplate: '{summary}',
      repo: 'tagoro9/fotingo',
    };
  },
  createTrackerConfig(): TrackerConfig {
    return {
      root: faker.internet.url(),
      status: {
        [IssueStatus.BACKLOG]: /backlog/i,
        [IssueStatus.DONE]: /done/i,
        [IssueStatus.IN_PROGRESS]: /progress/i,
        [IssueStatus.SELECTED_FOR_DEVELOPMENT]: /to do/i,
        [IssueStatus.IN_REVIEW]: /review/i,
      },
      user: {
        login: faker.internet.email(),
        token: faker.internet.password(),
      },
    };
  },
  createReleaseConfig(): ReleaseConfig {
    return {
      template:
        '{version}\n\n{fixedIssuesByCategory}\n\nSee [Jira release]({jira.release})\n\n{fotingo.banner}',
    };
  },
  createJiraUser(): User {
    return {
      accountId: faker.internet.userName(),
      displayName: faker.name.firstName(),
      groups: {
        items: {
          name: 'asda',
        },
      },
    };
  },
  createRelease(): Release {
    return {
      id: faker.random.word(),
      issues: [data.createIssue(IssueType.BUG), data.createIssue(IssueType.FEATURE)],
      name: faker.random.word(),
      url: faker.internet.url(),
    };
  },
  createHttpResponse<T>(body: T): HttpResponse<T> {
    return {
      body,
      response: {},
    };
  },
};
