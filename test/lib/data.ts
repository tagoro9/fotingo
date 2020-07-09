import * as faker from 'faker';
import { GitConfig } from 'src/git/Config';
import { JiraConfig } from 'src/issue-tracker/jira/Config';
import { JiraIssue } from 'src/issue-tracker/jira/types';
import { HttpResponse } from 'src/network/HttpClient';
import { Issue, IssueType, Release, ReleaseConfig, User } from 'src/types';

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
        },
        summary: overrides.summary || defaultSummary,
      },
      id: faker.random.number(5000),
      key: `FOTINGO-${faker.random.number(5000)}`,
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
      id: faker.random.number(5000),
      key: `FOTINGO-${faker.random.number(5000)}`,
      project: faker.lorem.word(),
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
  createJiraConfig(): JiraConfig {
    return {
      root: faker.internet.url(),
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
