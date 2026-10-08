import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { filterMemos, type MemoItem } from './memoSearch.ts';

describe('memoSearch utility', () => {
  const sampleMemos: MemoItem[] = [
    {
      id: 'm-1',
      type: 'text',
      title: 'Architectural Review',
      content: 'Refactor golden ratio layout to 4-pixel grid alignment',
      tags: ['architecture', 'layout'],
      createdAt: '2026-10-06T10:00:00Z',
    },
    {
      id: 'm-2',
      type: 'text',
      title: 'Database Migration',
      content: 'Run sqlite migrations for telemetry and session tokens',
      tags: ['database', 'backend'],
      createdAt: '2026-10-06T11:00:00Z',
    },
    {
      id: 'm-3',
      type: 'voice',
      title: 'Voice Memo: Audio Standup',
      content: 'Recorded standup discussion regarding Web Speech API',
      transcript: 'Web Speech API integration delivers real-time voice transcription',
      tags: ['voice', 'standup'],
      createdAt: '2026-10-06T12:00:00Z',
      duration: '0:35',
    },
    {
      id: 'm-4',
      type: 'audio',
      title: 'Quick Voice Snippet',
      content: 'Voice Recording #4 (12s)',
      transcript: 'Customer feedback on the toolbar button layout and symmetry',
      tags: ['voice', 'feedback'],
      created_at: '2026-10-06T13:00:00Z',
      duration: '0:12',
    },
    {
      id: 'm-5',
      type: 'text',
      title: 'Release Notes v2.4',
      content: 'Features include equalized button widths and in-panel search',
      tags: ['release', 'v2.4'],
      createdAt: '2026-10-06T14:00:00Z',
    },
  ];

  describe('Empty and Whitespace Queries', () => {
    it('returns all memos when query is an empty string', () => {
      const results = filterMemos(sampleMemos, '');
      assert.equal(results.length, sampleMemos.length);
      assert.deepEqual(results, sampleMemos);
    });

    it('returns all memos when query is whitespace only', () => {
      const results = filterMemos(sampleMemos, '   \t  \n  ');
      assert.equal(results.length, sampleMemos.length);
    });

    it('handles undefined or null array gracefully', () => {
      // @ts-expect-error test invalid parameter
      assert.deepEqual(filterMemos(null, 'test'), []);
      // @ts-expect-error test invalid parameter
      assert.deepEqual(filterMemos(undefined, 'test'), []);
    });
  });

  describe('Keyword Matching Across Fields', () => {
    it('matches by title keyword in text memos', () => {
      const results = filterMemos(sampleMemos, 'Architectural', 'text');
      assert.equal(results.length, 1);
      assert.equal(results[0].id, 'm-1');
    });

    it('matches by content keyword in text memos', () => {
      const results = filterMemos(sampleMemos, 'sqlite', 'text');
      assert.equal(results.length, 1);
      assert.equal(results[0].id, 'm-2');
    });

    it('matches by tag keyword in text memos', () => {
      const results = filterMemos(sampleMemos, 'layout', 'text');
      assert.equal(results.length, 1);
      assert.equal(results[0].id, 'm-1');
    });

    it('matches by transcript keyword when searchScope is "all"', () => {
      const results = filterMemos(sampleMemos, 'real-time voice transcription', 'all');
      assert.equal(results.length, 1);
      assert.equal(results[0].id, 'm-3');
    });

    it('returns an empty array when no memo matches query', () => {
      const results = filterMemos(sampleMemos, 'nonexistent-term-xyz123', 'all');
      assert.equal(results.length, 0);
    });
  });

  describe('Search Scope Isolation (text vs all)', () => {
    it('excludes voice and audio memos when searchScope is "text" (default)', () => {
      // Query "feedback" matches m-4 transcript, but m-4 is audio
      const textOnly = filterMemos(sampleMemos, 'feedback', 'text');
      assert.equal(textOnly.length, 0);

      // Query "Speech" matches m-3 content and transcript, but m-3 is voice
      const speechTextOnly = filterMemos(sampleMemos, 'Speech');
      assert.equal(speechTextOnly.length, 0);

      // Query "toolbar button" appears in m-4 transcript and m-5 content:
      // m-4 is audio, m-5 is text
      const buttonMatches = filterMemos(sampleMemos, 'button', 'text');
      assert.equal(buttonMatches.length, 1);
      assert.equal(buttonMatches[0].id, 'm-5');
    });

    it('includes voice and audio memos when searchScope is "all"', () => {
      const allResults = filterMemos(sampleMemos, 'feedback', 'all');
      assert.equal(allResults.length, 1);
      assert.equal(allResults[0].id, 'm-4');

      const speechAll = filterMemos(sampleMemos, 'Speech', 'all');
      assert.equal(speechAll.length, 1);
      assert.equal(speechAll[0].id, 'm-3');

      // Both m-4 (voice) and m-5 (text) mention button
      const buttonMatches = filterMemos(sampleMemos, 'button', 'all');
      assert.equal(buttonMatches.length, 2);
      const ids = buttonMatches.map((m) => m.id);
      assert.ok(ids.includes('m-4'));
      assert.ok(ids.includes('m-5'));
    });
  });

  describe('Case-Insensitivity & Whitespace Trimming', () => {
    it('performs case-insensitive matching', () => {
      const lowercase = filterMemos(sampleMemos, 'architectural', 'text');
      const uppercase = filterMemos(sampleMemos, 'ARCHITECTURAL', 'text');
      const mixed = filterMemos(sampleMemos, 'ArChItEcTuRaL', 'text');

      assert.equal(lowercase.length, 1);
      assert.equal(uppercase.length, 1);
      assert.equal(mixed.length, 1);
      assert.equal(lowercase[0].id, 'm-1');
      assert.equal(uppercase[0].id, 'm-1');
      assert.equal(mixed[0].id, 'm-1');
    });

    it('trims leading and trailing whitespace from query', () => {
      const trimmed = filterMemos(sampleMemos, '   sqlite   ', 'text');
      assert.equal(trimmed.length, 1);
      assert.equal(trimmed[0].id, 'm-2');
    });
  });
});
