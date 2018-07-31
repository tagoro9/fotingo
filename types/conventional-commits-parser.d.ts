declare module 'conventional-commits-parser' {
  import { Stream } from 'stream';

  export interface CommitReference {
    action: string;
    issue: string;
    raw: string;
    prefix: string;
  }

  interface CommitNote {
    title: string;
    text: string;
  }

  export interface ParsedCommit {
    type?: string;
    scope?: string;
    subject?: string;
    header: string;
    body?: string;
    footer?: string;
    notes: string[];
    references: CommitReference[];
    mentions: string[];
  }

  export function sync(message: string): ParsedCommit;
}
