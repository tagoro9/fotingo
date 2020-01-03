declare module 'conventional-commits-parser' {
  import { Stream } from 'stream';

  export interface CommitReference {
    action: string;
    issue: string;
    prefix: string;
    raw: string;
  }

  interface CommitNote {
    text: string;
    title: string;
  }

  export interface ParsedCommit {
    body?: string;
    footer?: string;
    header: string;
    mentions: string[];
    notes: string[];
    references: CommitReference[];
    scope?: string;
    subject?: string;
    type?: string;
  }

  export function sync(message: string): ParsedCommit;
}
